"""Dump libclient.so / libscplugin.so exports and Java_* symbols.

Writes captures/libclient_symbols.txt (gitignored).

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
OUT = OUT_DIR / "libclient_symbols.txt"


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
var mods = ['libclient.so', 'libscplugin.so'];
mods.forEach(function (mn) {
  try {
    var m = Process.getModuleByName(mn);
    var exps = m.enumerateExports().map(function (s) { return s.name; });
    var syms = m.enumerateSymbols().map(function (s) { return s.name; });
    send({mod: mn, exports: exps.length, symbols: syms.length,
          exportNames: exps.slice(0, 400),
          localNames: syms.filter(function(n){ return n.indexOf('Java_') === 0; }).slice(0, 400)});
  } catch (e) { send({mod: mn, err: e.toString()}); }
});
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
            out.write(str(p) + "\n")
            print(
                "MOD", p.get("mod"), "exports=", p.get("exports"),
                "symbols=", p.get("symbols"), "err=", p.get("err"), flush=True,
            )
            for n in (p.get("exportNames") or []):
                out.write("  EXPORT " + n + "\n")
            for n in (p.get("localNames") or []):
                out.write("  JAVA   " + n + "\n")
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
