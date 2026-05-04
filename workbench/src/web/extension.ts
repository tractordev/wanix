
import * as vscode from 'vscode';
import { WanixBridge } from './bridge.js';
//@ts-ignore
import { WanixFS } from "../wanix/fs.js";

declare const navigator: unknown;

type Config = {
	term?: boolean;
	ns?: {
		task: string;
		term: string;
	}
	shell?: {
		cmd: string;
		type: string;
		wd: string;
	}
}

export async function activate(context: vscode.ExtensionContext) {
	if (typeof navigator !== 'object') {	// do not run under node.js
		console.error("not running in browser");
		return;
	}
	
	let config: Config = {};
	const channel = new MessageChannel();
	const wanix = new Promise((resolve) => {
		channel.port2.onmessage = async (event) => {
			if (event.data.wanix) {
				config = event.data.config;
				resolve(new WanixFS(event.data.wanix));
			}
		}
	});
	const bridge = new WanixBridge(wanix, "");
	context.subscriptions.push(bridge);

	const port = (context as any).messagePassingProtocol;
	port.postMessage({type: "_port", port: channel.port1}, [channel.port1]);

	bridge.ready.then((fsys) => {
		if (config.shell) {
			context.subscriptions.push(vscode.commands.registerCommand('workbench.createTerminal', async () => {
				const term = vscode.window.createTerminal({ 
					name: 'Shell', 
					pty: await createTerminal(fsys, config)
				});
				term.show();
				context.subscriptions.push(term);
			}));
			if (config.term) {
				vscode.commands.executeCommand(`workbench.createTerminal`);
			}
			
		}
	});
	
	console.log('System extension activated');
}

async function createTerminal(fsys: any, config: Config) {
	const termID = (await fsys.readText(`${config.ns?.term}/new`)).trim();
    const termPath = [config.ns?.term, termID].join("/");
	const taskID = (await fsys.readText(`${config.ns?.task}/new/${config.shell?.type}`)).trim();
	const taskPath = [config.ns?.task, taskID].join("/");
	await fsys.writeFile(`${taskPath}/cmd`, config.shell?.cmd);
	await fsys.writeFile(`${taskPath}/dir`, config.shell?.wd);
	await fsys.writeFile(`${taskPath}/ctl`, `bind ${termPath}/program ${taskPath}/fd/0`);
	await fsys.writeFile(`${taskPath}/ctl`, `bind ${termPath}/program ${taskPath}/fd/1`);
	await fsys.writeFile(`${taskPath}/ctl`, `bind ${termPath}/program ${taskPath}/fd/2`);
	await fsys.writeFile(`${taskPath}/ctl`, "start");

	const writeEmitter = new vscode.EventEmitter<string>();
	const dec = new TextDecoder();
	const enc = new TextEncoder();
	const readable = await fsys.openReadable(`${termPath}/data`);
	const writable = (await fsys.openWritable(`${termPath}/data`)).getWriter();
	let buffer = '';
	return {
		onDidWrite: writeEmitter.event,
		open: () => {
			(async () => {
				for await (const chunk of readable) {
					writeEmitter.fire(dec.decode(chunk));
				}
			})();
		},
		close: () => {
			writable.close();
		},
		handleInput: async (data: string) => {
			// may add line discipline as mode to terminals but for now we
			// do as plan 9 and handle it here in "userspace"
			if (data === '\r') {
				writeEmitter.fire('\r\n');           // echo newline
				writable.write(enc.encode(buffer+"\n"));
				buffer = '';
			} else if (data === '\x7f') {   // backspace
				if (buffer.length > 0) {
					buffer = buffer.slice(0, -1);
					writeEmitter.fire('\b \b');
				}
			} else {
				buffer += data;
				writeEmitter.fire(data);             // echo
			}
		},
		setDimensions: async (dimensions: vscode.TerminalDimensions) => {
			// const winch = (await fsys.openWritable(`${termPath}/winch`)).getWriter();
			// await winch.write(enc.encode(`${dimensions.columns} ${dimensions.rows}\n`));
			// await winch.close();
		}
	};
}

// @ts-ignore
// polyfill for ReadableStream.prototype[Symbol.asyncIterator] on safari
if (!ReadableStream.prototype[Symbol.asyncIterator]) {
	// @ts-ignore
    ReadableStream.prototype[Symbol.asyncIterator] = async function* () {
        const reader = this.getReader();
        try {
            while (true) {
                const { done, value } = await reader.read();
                if (done) return;
                yield value;
            }
        } finally {
            reader.releaseLock();
        }
    };
}
