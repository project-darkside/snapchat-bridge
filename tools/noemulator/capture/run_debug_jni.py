"""Attach to Snapchat and run hook_debug_jni.js.

Dumps the JNIEnv function table, raw argument pointers, and a couple of
diagnostic JNI reads for NetworkApi.native_submit. Useful for verifying the
JNI function-table indices and Djinni CppProxy argument layout.

Environment:
  ADB, ANDROID_SERIAL, FRIDA_HOST as in run_grpc_all.py.
"""
import frida
import os
import subprocess
import sys
import time
from pathlib import Path

ADB = os.environ.get("ADB", "adb")
SERIAL = os.environ.get("ANDROID_SERIAL", "127.0.0.1:15555")
FRIDA_HOST = os.environ.get("FRIDA_HOST", "127.0.0.1:27043")
PKG = "com.snapchat.android"
JS = str(Path(__file__).with_name("hook_debug_jni.js"))
OUT_DIR = Path(__file__).with_name("captures")
OUT = OUT_DIR / "jni_debug.txt"
DURATION = float(sys.argv[1]) if len(sys.argv) > 1 else 60.0


def pid_of():
    out = subprocess.run(
        [ADB, "-s", SERIAL, "shell", "pidof", PKG],
        capture_output=True, text=True,
    ).stdout.strip()
    return int(out.split()[0]) if out else None


def main():
    pid = pid_of()
    if not pid:
        print("Snapchat is not running", flush=True)
        return 1
    print("pid", pid, flush=True)

    OUT_DIR.mkdir(exist_ok=True)
    source = Path(JS).read_text(encoding="utf-8")

    dm = frida.get_device_manager()
    dev = dm.add_remote_device(FRIDA_HOST)
    session = dev.attach(pid)
    script = session.create_script(source)
    out = open(OUT, "a", encoding="utf-8")

    def on_msg(msg, data):
        if msg.get("type") == "send":
            payload = msg["payload"]
            out.write(str(payload) + "\n")
            out.flush()
            print("EVT", payload, flush=True)
        elif msg.get("type") == "error":
            print("ERR", msg, flush=True)

    script.on("message", on_msg)
    script.load()
    print(f"capturing {DURATION}s to {OUT}", flush=True)
    deadline = time.time() + DURATION
    while time.time() < deadline:
        time.sleep(1)
    out.close()
    print("DONE", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
