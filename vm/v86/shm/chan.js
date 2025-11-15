
export class SharedMemoryChannel {
    constructor(emulator, baseAddr = 0x3F000000, size = 0x1000000) { // 16MB (was 1MB)
        this.emulator = emulator;
        this.baseAddr = baseAddr;
        this.size = size;
        
        // Memory layout
        this.CONTROL_SIZE = 0x1000;
        this.BUFFER_SIZE = (size - this.CONTROL_SIZE) / 2;
        
        // Control structure offsets (matching Go side)
        this.JS_GO_HEAD = 0;    // uint32 - JS writes here
        this.JS_GO_TAIL = 4;    // uint32 - Go reads from here
        this.GO_JS_HEAD = 8;    // uint32 - Go writes here
        this.GO_JS_TAIL = 12;   // uint32 - JS reads from here
        
        // Buffer offsets
        this.JS_GO_BUFFER = this.CONTROL_SIZE;
        this.GO_JS_BUFFER = this.CONTROL_SIZE + this.BUFFER_SIZE;
        
        // MessageChannel setup - port2 is for user code
        this.channel = new MessageChannel();
        this.port1 = this.channel.port1;
        this.port2 = this.channel.port2;
        
        // Handle messages from user (via port2) -> forward to Go
        this.port1.onmessage = (e) => this.write(e.data);
        
        // console.log("Starting polling for Go->JS messages");
        this.pollForMessages();
    }
    
    // Read uint32 little-endian from memory
    readUint32(offset) {
        const bytes = this.emulator.read_memory(this.baseAddr + offset, 4);
        return bytes[0] | (bytes[1] << 8) | (bytes[2] << 16) | (bytes[3] << 24);
    }
    
    // Write uint32 little-endian to memory
    writeUint32(offset, value) {
        const bytes = new Uint8Array(4);
        bytes[0] = value & 0xFF;
        bytes[1] = (value >> 8) & 0xFF;
        bytes[2] = (value >> 16) & 0xFF;
        bytes[3] = (value >> 24) & 0xFF;
        this.emulator.write_memory(bytes, this.baseAddr + offset);
    }
    
    pollForMessages() {
        // Tight polling loop for maximum throughput
        const poll = () => {
            // Read ALL available messages in one poll cycle
            let messagesRead = 0;
            const maxPerCycle = 1000; // Prevent infinite loop
            
            while (messagesRead < maxPerCycle) {
                // Read head and tail pointers
                const head = this.readUint32(this.GO_JS_HEAD);
                const tail = this.readUint32(this.GO_JS_TAIL);
                
                // Use unsigned subtraction to handle uint32 overflow correctly
                // >>> 0 ensures uint32 semantics in JavaScript
                const dataAvailable = ((head - tail) >>> 0);
                
                // Check if there's a message
                if (dataAvailable === 0) {
                    break; // No more messages
                }
                
                // Calculate buffer positions using modulo
                const tailPos = tail % this.BUFFER_SIZE;
                
                // Read message length at tail position (using modulo)
                if (tailPos + 4 > this.BUFFER_SIZE) {
                    // Not enough space for length header, skip to next cycle
                    const newTail = (Math.floor(tail / this.BUFFER_SIZE) + 1) * this.BUFFER_SIZE;
                    this.writeUint32(this.GO_JS_TAIL, newTail);
                    continue;
                }
                
                const lengthBytes = this.emulator.read_memory(
                    this.baseAddr + this.GO_JS_BUFFER + tailPos,
                    4
                );
                const msgLength = lengthBytes[0] | (lengthBytes[1] << 8) | 
                                (lengthBytes[2] << 16) | (lengthBytes[3] << 24);
                
                if (msgLength > 0 && msgLength < this.BUFFER_SIZE) {
                    // Check if full message fits in buffer
                    if (tailPos + 4 + msgLength <= this.BUFFER_SIZE) {
                        // Read message body
                        const messageBody = this.emulator.read_memory(
                            this.baseAddr + this.GO_JS_BUFFER + tailPos + 4,
                            msgLength
                        );
                        
                        // Update tail pointer (using logical position, not modulo)
                        const newTail = tail + 4 + msgLength;
                        this.writeUint32(this.GO_JS_TAIL, newTail);
                        
                        // Post to port1 so user's port2 receives it
                        this.port1.postMessage(messageBody.slice().buffer);
                        // console.log("outer: recv:", msgLength)

                        messagesRead++;
                    } else {
                        // Message doesn't fit at current position, must be a gap
                        const newTail = (Math.floor(tail / this.BUFFER_SIZE) + 1) * this.BUFFER_SIZE;
                        console.log(`Message doesn't fit (tailPos=${tailPos}, length=${msgLength}), skipping to ${newTail}`);
                        this.writeUint32(this.GO_JS_TAIL, newTail);
                    }
                } else {
                    // Invalid length, skip to next cycle
                    const newTail = (Math.floor(tail / this.BUFFER_SIZE) + 1) * this.BUFFER_SIZE;
                    // console.log(`Invalid message length: ${msgLength} at tailPos=${tailPos}, skipping to ${newTail}`);
                    this.writeUint32(this.GO_JS_TAIL, newTail);
                    break;
                }
            }
            
            // Immediate retry for maximum throughput
            setTimeout(poll, 0);
        };
        
        poll(); // Start polling
    }
    
    async write(data) {
        let bytes;
        if (typeof data === 'string') {
            bytes = new TextEncoder().encode(data);
        } else if (data instanceof ArrayBuffer) {
            bytes = new Uint8Array(data);
        } else if (data instanceof Uint8Array) {
            bytes = data;
        } else {
            bytes = new TextEncoder().encode(JSON.stringify(data));
        }
        
        const messageSize = 4 + bytes.length;
        
        // Busy-wait for immediate retries with long timeout
        for (let busyTries = 0; busyTries < 10000; busyTries++) {
            let head = this.readUint32(this.JS_GO_HEAD);
            const tail = this.readUint32(this.JS_GO_TAIL);
            
            // Calculate available space (BufferSize - used space)
            const safetyMargin = 16384;
            const dataUsed = ((head - tail) >>> 0);
            let available = this.BUFFER_SIZE - dataUsed;
            
            if (available > safetyMargin) {
                available -= safetyMargin;
            } else {
                available = 0;
            }
            
            // Check if we have space
            if (messageSize <= available) {
                // Calculate buffer position using modulo
                let headPos = head % this.BUFFER_SIZE;
                
                // Check if message fits at current position
                if (headPos + messageSize > this.BUFFER_SIZE) {
                    // Need to skip gap - advance head to next buffer cycle
                    const newHead = (Math.floor(head / this.BUFFER_SIZE) + 1) * this.BUFFER_SIZE;
                    const tailPos = tail % this.BUFFER_SIZE;
                    
                    // Check if tail is far enough from start of buffer (using buffer positions only)
                    // This avoids incorrect comparisons when uint32 overflows
                    if (tailPos >= messageSize) {
                        // Safe to skip gap and write at beginning
                        head = newHead;
                        headPos = 0;
                    } else {
                        // Not enough space at beginning, retry
                        continue;
                    }
                }
                
                // Write the message
                const message = new Uint8Array(messageSize);
                message[0] = bytes.length & 0xFF;
                message[1] = (bytes.length >> 8) & 0xFF;
                message[2] = (bytes.length >> 16) & 0xFF;
                message[3] = (bytes.length >> 24) & 0xFF;
                message.set(bytes, 4);
                
                this.emulator.write_memory(message, this.baseAddr + this.JS_GO_BUFFER + headPos);

                // Update head pointer (using logical position, not modulo)
                const newHead = head + messageSize;
                this.writeUint32(this.JS_GO_HEAD, newHead);

                // console.log("outer: write:", messageSize)
                return; // Success!
            }
            
			await new Promise(resolve => setTimeout(resolve, 10));
        }
        
        // After 100k tries (~few seconds), give up
        console.warn("Buffer full after 10k attempts - message dropped");
    }
    
    getUserPort() {
        return this.port2;  // User code uses port2 for bidirectional communication
    }
}
