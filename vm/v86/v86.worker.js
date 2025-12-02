
import { V86 } from "./libv86.mjs";
import { WanixHandle } from "../../runtime/assets/wanix.handle.js";
import { SharedMemoryChannel } from "./shm/chan.js";


self.onmessage = async (e) => {
	if (!e.data.id) {
		return;
	}
	console.log("v86 loading...");
	const id = e.data.id;
	const p9 = e.data.p9;
	const serial = e.data.serial;
	const fsys = new WanixHandle(e.data.sys);
	const seabios = await fsys.readFile("#bundle/v86/seabios.bin");
	const vgabios = await fsys.readFile("#bundle/v86/vgabios.bin");
	const bzimage = await fsys.readFile("#bundle/kernel/bzImage");
	const v86wasm = await fsys.readFile("#bundle/v86/v86.wasm");
	const wasmBlob = new Blob([v86wasm], { type: 'application/wasm' });

	let screenContainer = undefined;
	if (e.data.screen && document) {
		screenContainer = document.querySelector(e.data.screen);
	}
	
	// Store the send callback outside handle9p to avoid creating multiple handlers
	let p9SendCallback = null;
	p9.onmessage = (e) => {
		if (p9SendCallback) {
			p9SendCallback(e.data);
		}
	};
	
    const vm = new V86(Object.assign(e.data.options, {
		disable_speaker: true,
		screen_container: screenContainer,
		wasm_path: URL.createObjectURL(wasmBlob),
		filesystem: {
			handle9p: (buf, send) => {
				p9SendCallback = send;
				p9.postMessage(buf);
			},
		},
		bios: {buffer: seabios.slice().buffer},
		vga_bios: {buffer: vgabios.slice().buffer},
		bzimage: {buffer: bzimage.slice().buffer},
		// todo: maybe use this instead of serial for console
		// virtio_console: true,
	}));
	vm.add_listener("emulator-ready", function() {
		const channel = new SharedMemoryChannel(vm);
		vm.shmPort = channel.getUserPort();
		self.postMessage({ shmPort: vm.shmPort }, [vm.shmPort]);
	});
	serial.onmessage = (e) => vm.serial_send_bytes(0, e.data);
	vm.add_listener("serial0-output-byte", (c) => serial.postMessage(c));
	if (globalThis.window) {
		window.vm = vm;
	}
}

//# sourceURL=vm.1