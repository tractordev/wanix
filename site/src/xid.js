const ENCODING = "0123456789abcdefghijklmnopqrstuv";

let counter;
let machineId;
let pid;

function init() {
  const b = crypto.getRandomValues(new Uint8Array(3));
  counter = (b[0] << 16) | (b[1] << 8) | b[2];
  machineId = crypto.getRandomValues(new Uint8Array(3));
  pid = crypto.getRandomValues(new Uint8Array(2));
}

function encode(id) {
  const dst = new Array(20);
  dst[19] = ENCODING[(id[11] << 4) & 0x1f];
  dst[18] = ENCODING[id[11] >> 1];
  dst[17] = ENCODING[(id[11] >> 6) | ((id[10] << 2) & 0x1f)];
  dst[16] = ENCODING[id[10] >> 3];
  dst[15] = ENCODING[id[9] & 0x1f];
  dst[14] = ENCODING[(id[9] >> 5) | ((id[8] << 3) & 0x1f)];
  dst[13] = ENCODING[id[8] >> 2];
  dst[12] = ENCODING[(id[8] >> 7) | ((id[7] << 1) & 0x1f)];
  dst[11] = ENCODING[(id[7] >> 4) | ((id[6] << 4) & 0x1f)];
  dst[10] = ENCODING[id[6] >> 1];
  dst[9] = ENCODING[(id[6] >> 6) | ((id[5] << 2) & 0x1f)];
  dst[8] = ENCODING[id[5] >> 3];
  dst[7] = ENCODING[id[4] & 0x1f];
  dst[6] = ENCODING[(id[4] >> 5) | ((id[3] << 3) & 0x1f)];
  dst[5] = ENCODING[id[3] >> 2];
  dst[4] = ENCODING[(id[3] >> 7) | ((id[2] << 1) & 0x1f)];
  dst[3] = ENCODING[(id[2] >> 4) | ((id[1] << 4) & 0x1f)];
  dst[2] = ENCODING[id[1] >> 1];
  dst[1] = ENCODING[(id[1] >> 6) | ((id[0] << 2) & 0x1f)];
  dst[0] = ENCODING[id[0] >> 3];
  return dst.join("");
}

export function xid() {
  if (!machineId) {
    init();
  }

  const id = new Uint8Array(12);
  const view = new DataView(id.buffer);
  view.setUint32(0, Math.floor(Date.now() / 1000), false);
  id.set(machineId, 4);
  id[7] = pid[0];
  id[8] = pid[1];
  counter = (counter + 1) & 0xffffff;
  id[9] = counter >> 16;
  id[10] = (counter >> 8) & 0xff;
  id[11] = counter & 0xff;
  return encode(id);
}
