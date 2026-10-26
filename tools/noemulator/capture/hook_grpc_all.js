// Hook gRPC UnifiedGrpcService.unaryCall/serverStreamingCall + both submits.
// Djinni CppProxy static natives: (nativeRef jlong, ...method args).
var JNI = {
  ExceptionClear: 17,
  GetObjectClass: 31,
  CallObjectMethod: 34,
  CallIntMethod: 49,
  GetMethodID: 33,
  GetFieldID: 94,
  GetObjectField: 95,
  NewByteArray: 176,
  GetArrayLength: 171,
  GetByteArrayElements: 184,
  ReleaseByteArrayElements: 192,
  GetStringUTFChars: 169,
};

function jniFn(env, idx) {
  return env.readPointer().add(idx * Process.pointerSize).readPointer();
}
function clearEx(env) {
  try {
    var fn = new NativeFunction(jniFn(env, JNI.ExceptionClear), 'void', ['pointer']);
    fn(env);
  } catch (e) {}
}
function jstring(env, s) {
  if (s.isNull()) return null;
  try {
    var fn = new NativeFunction(jniFn(env, JNI.GetStringUTFChars), 'pointer', ['pointer', 'pointer', 'pointer']);
    var p = fn(env, s, ptr(0));
    if (p.isNull()) { clearEx(env); return null; }
    var r = p.readUtf8String();
    clearEx(env);
    return r;
  } catch (e) { clearEx(env); return null; }
}
function getObjectClass(env, obj) {
  try {
    var fn = new NativeFunction(jniFn(env, JNI.GetObjectClass), 'pointer', ['pointer', 'pointer']);
    var c = fn(env, obj);
    return c.isNull() ? null : c;
  } catch (e) { clearEx(env); return null; }
}
function getFieldID(env, cls, name, sig) {
  try {
    var nameP = Memory.allocUtf8String(name);
    var sigP = Memory.allocUtf8String(sig);
    var fn = new NativeFunction(jniFn(env, JNI.GetFieldID), 'pointer', ['pointer', 'pointer', 'pointer', 'pointer']);
    var id = fn(env, cls, nameP, sigP);
    if (id.isNull()) { clearEx(env); return null; }
    return id;
  } catch (e) { clearEx(env); return null; }
}
function getObjectField(env, obj, fieldID) {
  try {
    var fn = new NativeFunction(jniFn(env, JNI.GetObjectField), 'pointer', ['pointer', 'pointer', 'pointer']);
    var v = fn(env, obj, fieldID);
    clearEx(env);
    return v;
  } catch (e) { clearEx(env); return null; }
}
function getMethodID(env, cls, name, sig) {
  try {
    var nameP = Memory.allocUtf8String(name);
    var sigP = Memory.allocUtf8String(sig);
    var fn = new NativeFunction(jniFn(env, JNI.GetMethodID), 'pointer', ['pointer', 'pointer', 'pointer', 'pointer']);
    var id = fn(env, cls, nameP, sigP);
    if (id.isNull()) { clearEx(env); return null; }
    return id;
  } catch (e) { clearEx(env); return null; }
}
function callObject(env, obj, methodID) {
  try {
    var fn = new NativeFunction(jniFn(env, JNI.CallObjectMethod), 'pointer', ['pointer', 'pointer', 'pointer']);
    var v = fn(env, obj, methodID);
    clearEx(env);
    return v;
  } catch (e) { clearEx(env); return null; }
}
function callObject1(env, obj, methodID, arg1) {
  try {
    var fn = new NativeFunction(jniFn(env, JNI.CallObjectMethod), 'pointer', ['pointer', 'pointer', 'pointer', 'pointer']);
    var v = fn(env, obj, methodID, arg1);
    clearEx(env);
    return v;
  } catch (e) { clearEx(env); return null; }
}
function callInt(env, obj, methodID) {
  try {
    var fn = new NativeFunction(jniFn(env, JNI.CallIntMethod), 'int32', ['pointer', 'pointer', 'pointer']);
    var v = fn(env, obj, methodID);
    clearEx(env);
    return v;
  } catch (e) { clearEx(env); return -1; }
}
function newByteArray(env, len) {
  try {
    var fn = new NativeFunction(jniFn(env, JNI.NewByteArray), 'pointer', ['pointer', 'int32']);
    var a = fn(env, len);
    clearEx(env);
    return a;
  } catch (e) { clearEx(env); return null; }
}
function callObject1Int(env, obj, methodID, intArg) {
  try {
    var fn = new NativeFunction(jniFn(env, JNI.CallObjectMethod), 'pointer', ['pointer', 'pointer', 'pointer', 'int32']);
    var v = fn(env, obj, methodID, intArg);
    clearEx(env);
    return v;
  } catch (e) { clearEx(env); return null; }
}
// read a java.nio.ByteBuffer's full content (position 0 .. limit), heap or direct.
function readByteBuffer(env, bb) {
  if (bb.isNull()) return 'NULL';
  var cls = getObjectClass(env, bb);
  if (!cls) return 'NOCLASS';
  var limitID = getMethodID(env, cls, 'limit', '()I');
  if (!limitID) return 'NOLIMITID';
  var limit = callInt(env, bb, limitID);
  if (limit <= 0 || limit > 4 * 1024 * 1024) return 'LIMIT=' + limit;
  // position(0) so get() reads from the start
  var posID = getMethodID(env, cls, 'position', '(I)Ljava/nio/Buffer;');
  if (!posID) return 'NOPOSID';
  callObject1Int(env, bb, posID, 0);
  var dst = newByteArray(env, limit);
  if (!dst) return 'NODST';
  var getID = getMethodID(env, cls, 'get', '([B)Ljava/nio/ByteBuffer;');
  if (!getID) return 'NOGETID';
  var ret = callObject1(env, bb, getID, dst);
  if (ret.isNull()) return 'GETNULL';
  try {
    var elemsFn = new NativeFunction(jniFn(env, JNI.GetByteArrayElements), 'pointer', ['pointer', 'pointer', 'pointer']);
    var relFn = new NativeFunction(jniFn(env, JNI.ReleaseByteArrayElements), 'void', ['pointer', 'pointer', 'pointer', 'int32']);
    var elems = elemsFn(env, dst, ptr(0));
    if (elems.isNull()) { clearEx(env); return 'NOELEMS'; }
    var bytes = elems.readByteArray(limit);
    relFn(env, dst, elems, 2); // JNI_ABORT
    clearEx(env);
    var u8 = new Uint8Array(bytes);
    var hex = [];
    for (var i = 0; i < u8.length; i++) hex.push(('0' + u8[i].toString(16)).slice(-2));
    return { len: u8.length, hex: hex.join('') };
  } catch (e) { clearEx(env); return 'ERR ' + e; }
}
function readUploadDataProvider(env, udp) {
  if (udp.isNull()) return null;
  var cls = getObjectClass(env, udp);
  if (!cls) return null;
  var getInMemID = getMethodID(env, cls, 'getUploadInMemoryDataProvider', '()Lcom/snapchat/client/network_types/UploadInMemoryDataProvider;');
  if (!getInMemID) return null;
  var inMem = callObject(env, udp, getInMemID);
  if (inMem.isNull()) return null;
  var inMemCls = getObjectClass(env, inMem);
  if (!inMemCls) return null;
  var dataID = getMethodID(env, inMemCls, 'data', '()Lcom/snapchat/client/shims/DataProvider;');
  if (!dataID) return null;
  var dp = callObject(env, inMem, dataID);
  if (dp.isNull()) return null;
  var dpCls = getObjectClass(env, dp);
  if (!dpCls) return null;
  var bbID = getMethodID(env, dpCls, 'data', '()Ljava/nio/ByteBuffer;');
  if (!bbID) return null;
  var bb = callObject(env, dp, bbID);
  if (bb.isNull()) return null;
  return readByteBuffer(env, bb);
}
function readStringField(env, obj, fieldName) {
  var cls = getObjectClass(env, obj);
  if (!cls) return '<no class>';
  var id = getFieldID(env, cls, fieldName, 'Ljava/lang/String;');
  if (!id) return '<no ' + fieldName + ' fieldID>';
  var v = getObjectField(env, obj, id);
  var s = jstring(env, v);
  return s === null ? '<null str>' : s;
}
function attach(name, fn) {
  var mod = Process.getModuleByName('libclient.so');
  var addr = mod.findExportByName(name);
  if (!addr) { send({skip: name}); return; }
  Interceptor.attach(addr, { onEnter: fn });
  send({ready: name, addr: addr.toString()});
}

// gRPC unaryCall: (nativeRef, String path, ByteBuffer body, CallOptionsBuilder, UnaryEventHandler)
attach('Java_com_snapchat_client_grpc_UnifiedGrpcService_00024CppProxy_native_1unaryCall', function (args) {
  var env = args[0];
  send({kind: 'grpc_unaryCall', path: jstring(env, args[3]), body: readByteBuffer(env, args[4])});
});

// gRPC serverStreamingCall: (nativeRef, String path, ByteBuffer body, CallOptionsBuilder, ServerStreamingEventHandler)
attach('Java_com_snapchat_client_grpc_UnifiedGrpcService_00024CppProxy_native_1serverStreamingCall', function (args) {
  var env = args[0];
  send({kind: 'grpc_serverStreamingCall', path: jstring(env, args[3]), body: readByteBuffer(env, args[4])});
});

// gRPC bidiStreamingCall: (nativeRef, String path, CallOptionsBuilder, ServerStreamingEventHandler)
attach('Java_com_snapchat_client_grpc_UnifiedGrpcService_00024CppProxy_native_1bidiStreamingCall', function (args) {
  var env = args[0];
  send({kind: 'grpc_bidiStreamingCall', path: jstring(env, args[3])});
});

// NetworkApi.submit
attach('Java_com_snapchat_client_network_1api_NetworkApi_00024CppProxy_native_1submit', function (args) {
  var env = args[0];
  send({kind: 'networkapi_submit', url: readStringField(env, args[3], 'mUrl'), body: readUploadDataProvider(env, args[8])});
});

// NativeNetworkApi.submit
attach('Java_com_snapchat_client_native_1network_1api_NativeNetworkApi_00024CppProxy_native_1submit', function (args) {
  var env = args[0];
  send({kind: 'nativenetworkapi_submit', url: readStringField(env, args[3], 'mUrl'), body: readUploadDataProvider(env, args[6])});
});
