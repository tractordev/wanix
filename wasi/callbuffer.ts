import { decode, encode } from 'cbor-x';

export interface Caller {
    call(method: string, params: {}): any;
}

export interface Responder {
    respond(value: any): void;
}

export class CallBuffer {
    buffer: SharedArrayBuffer;
    ctrl: Int32Array;
    len: Int32Array;
    data: Uint8Array;
    maxData: number;

    constructor(buf) {
        this.buffer = buf;
        this.ctrl = new Int32Array(this.buffer, 0, 2);
        this.len = new Int32Array(this.buffer, 4, 1);
        this.data = new Uint8Array(this.buffer, 8);
        this.maxData = this.buffer.byteLength - 8 - 16/* cbor overhead */;
    }

    respond(value: any): void {
        let buf;
        if (value instanceof Uint8Array) {
            const limit = Math.min(value.length, this.maxData);
            buf = encode(value.slice(0, limit));
        } else {
            buf = encode(value);
        }
        // TODO: check if it exceeds the buffer size
        this.len[0] = buf.length;
        this.data.set(buf, 0);

        Atomics.store(this.ctrl, 0, 1);
        Atomics.notify(this.ctrl, 0);
    }

    call(method: string, params: {}): any {
        this.ctrl[0] = 0;
        params["method"] = method;
        postMessage(params);
        Atomics.wait(this.ctrl, 0, 0);
        return decode(this.data.slice(0, this.len[0]));
    }

}