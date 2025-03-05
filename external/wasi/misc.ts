import { File, OpenFile } from "@bjorn3/browser_wasi_shim";

export class EmptyFile extends File {
    constructor() {
        super([]);
    }
}

export class OpenEmptyFile extends OpenFile {
    constructor() {
        super(new EmptyFile());
    }
}
