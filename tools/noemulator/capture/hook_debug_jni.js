// Minimal debug: dump JNIEnv table pointers and try GetObjectClass/GetStringUTFChars.
var NAME = 'Java_com_snapchat_client_network_1api_NetworkApi_00024CppProxy_native_1submit';
var mod = Process.getModuleByName('libclient.so');
var addr = mod.findExportByName(NAME);
if (!addr) { send({skip: NAME}); } else {
  Interceptor.attach(addr, {
    onEnter: function (args) {
      var env = args[0];
      var out = {};
      out.env = env.toString();
      out.envIsNull = env.isNull();
      try {
        var table = env.readPointer();
        out.table = table.toString();
        var idxs = [17, 31, 33, 49, 94, 95, 169, 171, 176, 184, 192];
        var t = {};
        idxs.forEach(function (i) {
          try {
            var p = table.add(i * Process.pointerSize).readPointer();
            t[i] = p.isNull() ? 'null' : p.toString();
          } catch (e) { t[i] = 'ERR ' + e; }
        });
        out.tableIdx = t;
      } catch (e) { out.tableReadErr = String(e); }
      // raw arg pointers
      var a = {};
      for (var i = 0; i <= 10; i++) {
        try { a[i] = args[i].isNull() ? 'null' : args[i].toString(); } catch (e) { a[i] = 'ERR'; }
      }
      out.rawArgs = a;
      // try GetObjectClass(args[2])
      try {
        var fn = new NativeFunction(table.add(31 * Process.pointerSize).readPointer(), 'pointer', ['pointer', 'pointer']);
        var cls = fn(env, args[2]);
        out.getObjectClassArgs2 = cls.isNull() ? 'null' : cls.toString();
      } catch (e) { out.getObjectClassArgs2 = 'ERR ' + e; }
      // try GetStringUTFChars(args[3])
      try {
        var fn2 = new NativeFunction(table.add(169 * Process.pointerSize).readPointer(), 'pointer', ['pointer', 'pointer', 'pointer']);
        var p = fn2(env, args[3], ptr(0));
        out.str3 = p.isNull() ? 'null' : p.readUtf8String();
      } catch (e) { out.str3 = 'ERR ' + e; }
      send(out);
    }
  });
  send({ready: NAME});
}
