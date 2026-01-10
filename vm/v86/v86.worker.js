
import { V86 } from "./libv86.mjs";
import { OffscreenScreenAdapter } from "./offscreen.js";
import { WanixHandle } from "../../runtime/assets/wanix.handle.js";
import { SharedMemoryChannel } from "./shm/chan.js";


let vm = null;
const inputHandler = (e) => {
	const data = e.data;
	switch(data.type) {
		case "keyboard":
			vm.keyboard_send_scancodes(data.codes);
			break;
		case "mouse-click":
			vm.bus.send("mouse-click", [data.left, data.middle, data.right]);
			break;
		case "mouse-delta":
			vm.bus.send("mouse-delta", [data.deltaX, data.deltaY]);
			break;
		case "mouse-wheel":
			vm.bus.send("mouse-wheel", [data.deltaX, data.deltaY]);
			break;
	}
};
self.onmessage = async (e) => {
	if (!e.data.id) {
		return;
	}
	if (e.data.screen && vm) {
		vm.screen_adapter.set_canvas(e.data.screen);
		if (e.data.input) {
			e.data.input.onmessage = inputHandler;
		}
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

	
	// Store the send callback outside handle9p to avoid creating multiple handlers
	let p9SendCallback = null;
	p9.onmessage = (e) => {
		if (p9SendCallback) {
			p9SendCallback(e.data);
		}
	};
	
    vm = new V86(Object.assign(e.data.options, {
		disable_speaker: true,
		disable_mouse: true,
		disable_keyboard: false,
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

		// vm.bus.register("dac-send-data", (data) => {
		// 	self.postMessage({ audio: {left: data[0], right: data[1] } }, [data[0].buffer, data[1].buffer]);
		// 	// console.log("dac-send-data", data);
		// });
		// vm.bus.register("dac-tell-sampling-rate", (data) => {
		// 	self.postMessage({ audio: {rate: data} });
		// 	// console.log("dac-tell-sampling-rate", data);
		// });
		// setInterval(() => {
		// 	vm.bus.send("dac-request-data");
		// }, 20); // ~50 times per second


		if (e.data.screen) {
			const offscreenScreen = new OffscreenScreenAdapter(e.data.screen,
				() => vm.v86.cpu.devices.vga && vm.v86.cpu.devices.vga.screen_fill_buffer()
			);
			vm.v86.cpu.devices.vga.screen = offscreenScreen;
			vm.screen_adapter = offscreenScreen;

			if (e.data.input) {
				e.data.input.onmessage = inputHandler;
			}
		}
	});
	serial.onmessage = (e) => vm.serial_send_bytes(0, e.data);
	vm.add_listener("serial0-output-byte", (c) => serial.postMessage(c));
	if (globalThis.window) {
		window.vm = vm;
	}
}

//# sourceURL=vm.1