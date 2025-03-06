import { WASI, OpenFile } from "./index.ts";
import { wasi } from "@bjorn3/browser_wasi_shim";

// Workaround for https://github.com/bjorn3/browser_wasi_shim/issues/14
export function applyPatchPollOneoff(self: WASI): void {
  self.wasiImport.poll_oneoff = ((
    inPtr: number,
    outPtr: number,
    nsubscriptions: number,
    sizeOutPtr: number,
  ): number => {
    if (nsubscriptions < 0) {
      return wasi.ERRNO_INVAL;
    }

    const size_subscription = 48;
    const subscriptions = new DataView(
      self.inst.exports.memory.buffer,
      inPtr,
      nsubscriptions * size_subscription,
    );

    const size_event = 32;
    const events = new DataView(
      self.inst.exports.memory.buffer,
      outPtr,
      nsubscriptions * size_event,
    );

    for (let i = 0; i < nsubscriptions; ++i) {
      const subscription_userdata_offset = 0;
      const userdata = subscriptions.getBigUint64(
        i * size_subscription + subscription_userdata_offset,
        true,
      );

      const subscription_u_offset = 8;
      const subscription_u_tag = subscriptions.getUint8(
        i * size_subscription + subscription_u_offset,
      );
      const subscription_u_tag_size = 1;

      const event_userdata_offset = 0;
      const event_error_offset = 8;
      const event_type_offset = 10;
      const event_fd_readwrite_nbytes_offset = 16;
      const event_fd_readwrite_flags_offset = 16 + 8;

      events.setBigUint64(
        i * size_event + event_userdata_offset,
        userdata,
        true,
      );
      events.setUint32(
        i * size_event + event_error_offset,
        wasi.ERRNO_SUCCESS,
        true,
      );

      function assertOpenFileAvailable(): OpenFile {
        const fd = subscriptions.getUint32(
          i * size_subscription +
            subscription_u_offset +
            subscription_u_tag_size,
          true,
        );
        const openFile = self.fds[fd];
        if (!(openFile instanceof OpenFile)) {
          throw new Error(`FD#${fd} cannot be polled!`);
        }
        return openFile;
      }
      function setEventFdReadWrite(size: bigint): void {
        events.setUint16(
          i * size_event + event_type_offset,
          wasi.EVENTTYPE_FD_READ,
          true,
        );
        events.setBigUint64(
          i * size_event + event_fd_readwrite_nbytes_offset,
          size,
          true,
        );
        events.setUint16(
          i * size_event + event_fd_readwrite_flags_offset,
          0,
          true,
        );
      }
      switch (subscription_u_tag) {
        case wasi.EVENTTYPE_CLOCK:
          events.setUint16(
            i * size_event + event_type_offset,
            wasi.EVENTTYPE_CLOCK,
            true,
          );
          break;
        case wasi.EVENTTYPE_FD_READ:
          const fileR = assertOpenFileAvailable();
          setEventFdReadWrite(fileR.file.size);
          break;
        case wasi.EVENTTYPE_FD_WRITE:
          // I'm not sure why, but an unavailable (already closed) FD is referenced here. So don't call assertOpenFileAvailable.
          setEventFdReadWrite(1n << 31n);
          break;
        default:
          throw new Error(`Unknown event type: ${subscription_u_tag}`);
      }
    }

    const size_size = 4;
    const outNSize = new DataView(
      self.inst.exports.memory.buffer,
      sizeOutPtr,
      size_size,
    );
    outNSize.setUint32(0, nsubscriptions, true);
    return wasi.ERRNO_SUCCESS;
  }) as (...args: unknown[]) => unknown;
}