"""Filter libclient.so exports for network/gRPC/attestation-related names.

Writes captures/libclient_net_exports.txt (gitignored).

Environment:
  ADB, ANDROID_SERIAL, FRIDA_HOST as in run_grpc_all.py.
"""
import frida
import os
import subprocess
import time
from pathlib import Path

ADB = os.environ.get("ADB", "adb")
SERIAL = os.environ.get("ANDROID_SERIAL", "127.0.0.1:15555")
FRIDA_HOST = os.environ.get("FRIDA_HOST", "127.0.0.1:27043")
PKG = "com.snapchat.android"
OUT_DIR = Path(__file__).with_name("captures")
OUT = OUT_DIR / "libclient_net_exports.txt"


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
    dm = frida.get_device_manager()
    dev = dm.add_remote_device(FRIDA_HOST)
    session = dev.attach(pid)

    js = r"""
var m = Process.getModuleByName('libclient.so');
var exps = m.enumerateExports().map(function (s) { return s.name; });
var pat = /grpc|network|unary|stream|call|submit|attest|fidelius|login|auth|envelope|request|response|send|receive|rpc|channel|transport|saga/i;
var hits = exps.filter(function (n) { return pat.test(n); });
send({total_exports: exps.length, hit_count: hits.length, hits: hits});
send({done: true});
"""
    script = session.create_script(js)
    done = [False]
    out = open(OUT, "w", encoding="utf-8")

    def on_msg(msg, data):
        if msg.get("type") == "send":
            p = msg["payload"]
            if p.get("done"):
                done[0] = True
                return
            print(
                "total_exports=%s hit_count=%s" % (p.get("total_exports"), p.get("hit_count")),
                flush=True,
            )
            for n in p.get("hits", []):
                out.write(n + "\n")
                print(n, flush=True)
            out.flush()
        elif msg.get("type") == "error":
            print("ERR", msg, flush=True)
            done[0] = True

    script.on("message", on_msg)
    script.load()
    deadline = time.time() + 30
    while not done[0] and time.time() < deadline:
        time.sleep(0.5)
    out.close()
    print("DONE" if done[0] else "TIMEOUT", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
