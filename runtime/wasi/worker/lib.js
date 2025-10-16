var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __require = /* @__PURE__ */ ((x) => typeof require !== "undefined" ? require : typeof Proxy !== "undefined" ? new Proxy(x, {
  get: (a, b) => (typeof require !== "undefined" ? require : a)[b]
}) : x)(function(x) {
  if (typeof require !== "undefined") return require.apply(this, arguments);
  throw Error('Dynamic require of "' + x + '" is not supported');
});
var __commonJS = (cb, mod) => function __require2() {
  return mod || (0, cb[__getOwnPropNames(cb)[0]])((mod = { exports: {} }).exports, mod), mod.exports;
};
var __export = (target3, all) => {
  for (var name in all)
    __defProp(target3, name, { get: all[name], enumerable: true });
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toESM = (mod, isNodeMode, target3) => (target3 = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp(target3, "default", { value: mod, enumerable: true }) : target3,
  mod
));

// node_modules/ws/browser.js
var require_browser = __commonJS({
  "node_modules/ws/browser.js"(exports, module) {
    "use strict";
    module.exports = function() {
      throw new Error(
        "ws does not work in the browser. Browser clients must use the native WebSocket object"
      );
    };
  }
});

// node_modules/cbor-x/decode.js
var decoder;
try {
  decoder = new TextDecoder();
} catch (error) {
}
var src;
var srcEnd;
var position = 0;
var EMPTY_ARRAY = [];
var LEGACY_RECORD_INLINE_ID = 105;
var RECORD_DEFINITIONS_ID = 57342;
var RECORD_INLINE_ID = 57343;
var BUNDLED_STRINGS_ID = 57337;
var PACKED_REFERENCE_TAG_ID = 6;
var STOP_CODE = {};
var maxArraySize = 11281e4;
var maxMapSize = 1681e4;
var strings = EMPTY_ARRAY;
var stringPosition = 0;
var currentDecoder = {};
var currentStructures;
var srcString;
var srcStringStart = 0;
var srcStringEnd = 0;
var bundledStrings;
var referenceMap;
var currentExtensions = [];
var currentExtensionRanges = [];
var packedValues;
var dataView;
var restoreMapsAsObject;
var defaultOptions = {
  useRecords: false,
  mapsAsObjects: true
};
var sequentialMode = false;
var inlineObjectReadThreshold = 2;
try {
  new Function("");
} catch (error) {
  inlineObjectReadThreshold = Infinity;
}
var Decoder = class _Decoder {
  constructor(options) {
    if (options) {
      if ((options.keyMap || options._keyMap) && !options.useRecords) {
        options.useRecords = false;
        options.mapsAsObjects = true;
      }
      if (options.useRecords === false && options.mapsAsObjects === void 0)
        options.mapsAsObjects = true;
      if (options.getStructures)
        options.getShared = options.getStructures;
      if (options.getShared && !options.structures)
        (options.structures = []).uninitialized = true;
      if (options.keyMap) {
        this.mapKey = /* @__PURE__ */ new Map();
        for (let [k, v] of Object.entries(options.keyMap)) this.mapKey.set(v, k);
      }
    }
    Object.assign(this, options);
  }
  /*
  decodeKey(key) {
  	return this.keyMap
  		? Object.keys(this.keyMap)[Object.values(this.keyMap).indexOf(key)] || key
  		: key
  }
  */
  decodeKey(key) {
    return this.keyMap ? this.mapKey.get(key) || key : key;
  }
  encodeKey(key) {
    return this.keyMap && this.keyMap.hasOwnProperty(key) ? this.keyMap[key] : key;
  }
  encodeKeys(rec) {
    if (!this._keyMap) return rec;
    let map = /* @__PURE__ */ new Map();
    for (let [k, v] of Object.entries(rec)) map.set(this._keyMap.hasOwnProperty(k) ? this._keyMap[k] : k, v);
    return map;
  }
  decodeKeys(map) {
    if (!this._keyMap || map.constructor.name != "Map") return map;
    if (!this._mapKey) {
      this._mapKey = /* @__PURE__ */ new Map();
      for (let [k, v] of Object.entries(this._keyMap)) this._mapKey.set(v, k);
    }
    let res = {};
    map.forEach((v, k) => res[safeKey(this._mapKey.has(k) ? this._mapKey.get(k) : k)] = v);
    return res;
  }
  mapDecode(source, end) {
    let res = this.decode(source);
    if (this._keyMap) {
      switch (res.constructor.name) {
        case "Array":
          return res.map((r) => this.decodeKeys(r));
      }
    }
    return res;
  }
  decode(source, end) {
    if (src) {
      return saveState(() => {
        clearSource();
        return this ? this.decode(source, end) : _Decoder.prototype.decode.call(defaultOptions, source, end);
      });
    }
    srcEnd = end > -1 ? end : source.length;
    position = 0;
    stringPosition = 0;
    srcStringEnd = 0;
    srcString = null;
    strings = EMPTY_ARRAY;
    bundledStrings = null;
    src = source;
    try {
      dataView = source.dataView || (source.dataView = new DataView(source.buffer, source.byteOffset, source.byteLength));
    } catch (error) {
      src = null;
      if (source instanceof Uint8Array)
        throw error;
      throw new Error("Source must be a Uint8Array or Buffer but was a " + (source && typeof source == "object" ? source.constructor.name : typeof source));
    }
    if (this instanceof _Decoder) {
      currentDecoder = this;
      packedValues = this.sharedValues && (this.pack ? new Array(this.maxPrivatePackedValues || 16).concat(this.sharedValues) : this.sharedValues);
      if (this.structures) {
        currentStructures = this.structures;
        return checkedRead();
      } else if (!currentStructures || currentStructures.length > 0) {
        currentStructures = [];
      }
    } else {
      currentDecoder = defaultOptions;
      if (!currentStructures || currentStructures.length > 0)
        currentStructures = [];
      packedValues = null;
    }
    return checkedRead();
  }
  decodeMultiple(source, forEach) {
    let values, lastPosition = 0;
    try {
      let size = source.length;
      sequentialMode = true;
      let value = this ? this.decode(source, size) : defaultDecoder.decode(source, size);
      if (forEach) {
        if (forEach(value) === false) {
          return;
        }
        while (position < size) {
          lastPosition = position;
          if (forEach(checkedRead()) === false) {
            return;
          }
        }
      } else {
        values = [value];
        while (position < size) {
          lastPosition = position;
          values.push(checkedRead());
        }
        return values;
      }
    } catch (error) {
      error.lastPosition = lastPosition;
      error.values = values;
      throw error;
    } finally {
      sequentialMode = false;
      clearSource();
    }
  }
};
function checkedRead() {
  try {
    let result = read();
    if (bundledStrings) {
      if (position >= bundledStrings.postBundlePosition) {
        let error = new Error("Unexpected bundle position");
        error.incomplete = true;
        throw error;
      }
      position = bundledStrings.postBundlePosition;
      bundledStrings = null;
    }
    if (position == srcEnd) {
      currentStructures = null;
      src = null;
      if (referenceMap)
        referenceMap = null;
    } else if (position > srcEnd) {
      let error = new Error("Unexpected end of CBOR data");
      error.incomplete = true;
      throw error;
    } else if (!sequentialMode) {
      throw new Error("Data read, but end of buffer not reached");
    }
    return result;
  } catch (error) {
    clearSource();
    if (error instanceof RangeError || error.message.startsWith("Unexpected end of buffer")) {
      error.incomplete = true;
    }
    throw error;
  }
}
function read() {
  let token = src[position++];
  let majorType = token >> 5;
  token = token & 31;
  if (token > 23) {
    switch (token) {
      case 24:
        token = src[position++];
        break;
      case 25:
        if (majorType == 7) {
          return getFloat16();
        }
        token = dataView.getUint16(position);
        position += 2;
        break;
      case 26:
        if (majorType == 7) {
          let value = dataView.getFloat32(position);
          if (currentDecoder.useFloat32 > 2) {
            let multiplier = mult10[(src[position] & 127) << 1 | src[position + 1] >> 7];
            position += 4;
            return (multiplier * value + (value > 0 ? 0.5 : -0.5) >> 0) / multiplier;
          }
          position += 4;
          return value;
        }
        token = dataView.getUint32(position);
        position += 4;
        break;
      case 27:
        if (majorType == 7) {
          let value = dataView.getFloat64(position);
          position += 8;
          return value;
        }
        if (majorType > 1) {
          if (dataView.getUint32(position) > 0)
            throw new Error("JavaScript does not support arrays, maps, or strings with length over 4294967295");
          token = dataView.getUint32(position + 4);
        } else if (currentDecoder.int64AsNumber) {
          token = dataView.getUint32(position) * 4294967296;
          token += dataView.getUint32(position + 4);
        } else
          token = dataView.getBigUint64(position);
        position += 8;
        break;
      case 31:
        switch (majorType) {
          case 2:
          // byte string
          case 3:
            throw new Error("Indefinite length not supported for byte or text strings");
          case 4:
            let array = [];
            let value, i = 0;
            while ((value = read()) != STOP_CODE) {
              if (i >= maxArraySize) throw new Error(`Array length exceeds ${maxArraySize}`);
              array[i++] = value;
            }
            return majorType == 4 ? array : majorType == 3 ? array.join("") : Buffer.concat(array);
          case 5:
            let key;
            if (currentDecoder.mapsAsObjects) {
              let object = {};
              let i2 = 0;
              if (currentDecoder.keyMap) {
                while ((key = read()) != STOP_CODE) {
                  if (i2++ >= maxMapSize) throw new Error(`Property count exceeds ${maxMapSize}`);
                  object[safeKey(currentDecoder.decodeKey(key))] = read();
                }
              } else {
                while ((key = read()) != STOP_CODE) {
                  if (i2++ >= maxMapSize) throw new Error(`Property count exceeds ${maxMapSize}`);
                  object[safeKey(key)] = read();
                }
              }
              return object;
            } else {
              if (restoreMapsAsObject) {
                currentDecoder.mapsAsObjects = true;
                restoreMapsAsObject = false;
              }
              let map = /* @__PURE__ */ new Map();
              if (currentDecoder.keyMap) {
                let i2 = 0;
                while ((key = read()) != STOP_CODE) {
                  if (i2++ >= maxMapSize) {
                    throw new Error(`Map size exceeds ${maxMapSize}`);
                  }
                  map.set(currentDecoder.decodeKey(key), read());
                }
              } else {
                let i2 = 0;
                while ((key = read()) != STOP_CODE) {
                  if (i2++ >= maxMapSize) {
                    throw new Error(`Map size exceeds ${maxMapSize}`);
                  }
                  map.set(key, read());
                }
              }
              return map;
            }
          case 7:
            return STOP_CODE;
          default:
            throw new Error("Invalid major type for indefinite length " + majorType);
        }
      default:
        throw new Error("Unknown token " + token);
    }
  }
  switch (majorType) {
    case 0:
      return token;
    case 1:
      return ~token;
    case 2:
      return readBin(token);
    case 3:
      if (srcStringEnd >= position) {
        return srcString.slice(position - srcStringStart, (position += token) - srcStringStart);
      }
      if (srcStringEnd == 0 && srcEnd < 140 && token < 32) {
        let string = token < 16 ? shortStringInJS(token) : longStringInJS(token);
        if (string != null)
          return string;
      }
      return readFixedString(token);
    case 4:
      if (token >= maxArraySize) throw new Error(`Array length exceeds ${maxArraySize}`);
      let array = new Array(token);
      for (let i = 0; i < token; i++) array[i] = read();
      return array;
    case 5:
      if (token >= maxMapSize) throw new Error(`Map size exceeds ${maxArraySize}`);
      if (currentDecoder.mapsAsObjects) {
        let object = {};
        if (currentDecoder.keyMap) for (let i = 0; i < token; i++) object[safeKey(currentDecoder.decodeKey(read()))] = read();
        else for (let i = 0; i < token; i++) object[safeKey(read())] = read();
        return object;
      } else {
        if (restoreMapsAsObject) {
          currentDecoder.mapsAsObjects = true;
          restoreMapsAsObject = false;
        }
        let map = /* @__PURE__ */ new Map();
        if (currentDecoder.keyMap) for (let i = 0; i < token; i++) map.set(currentDecoder.decodeKey(read()), read());
        else for (let i = 0; i < token; i++) map.set(read(), read());
        return map;
      }
    case 6:
      if (token >= BUNDLED_STRINGS_ID) {
        let structure = currentStructures[token & 8191];
        if (structure) {
          if (!structure.read) structure.read = createStructureReader(structure);
          return structure.read();
        }
        if (token < 65536) {
          if (token == RECORD_INLINE_ID) {
            let length = readJustLength();
            let id = read();
            let structure2 = read();
            recordDefinition(id, structure2);
            let object = {};
            if (currentDecoder.keyMap) for (let i = 2; i < length; i++) {
              let key = currentDecoder.decodeKey(structure2[i - 2]);
              object[safeKey(key)] = read();
            }
            else for (let i = 2; i < length; i++) {
              let key = structure2[i - 2];
              object[safeKey(key)] = read();
            }
            return object;
          } else if (token == RECORD_DEFINITIONS_ID) {
            let length = readJustLength();
            let id = read();
            for (let i = 2; i < length; i++) {
              recordDefinition(id++, read());
            }
            return read();
          } else if (token == BUNDLED_STRINGS_ID) {
            return readBundleExt();
          }
          if (currentDecoder.getShared) {
            loadShared();
            structure = currentStructures[token & 8191];
            if (structure) {
              if (!structure.read)
                structure.read = createStructureReader(structure);
              return structure.read();
            }
          }
        }
      }
      let extension = currentExtensions[token];
      if (extension) {
        if (extension.handlesRead)
          return extension(read);
        else
          return extension(read());
      } else {
        let input = read();
        for (let i = 0; i < currentExtensionRanges.length; i++) {
          let value = currentExtensionRanges[i](token, input);
          if (value !== void 0)
            return value;
        }
        return new Tag(input, token);
      }
    case 7:
      switch (token) {
        case 20:
          return false;
        case 21:
          return true;
        case 22:
          return null;
        case 23:
          return;
        // undefined
        case 31:
        default:
          let packedValue = (packedValues || getPackedValues())[token];
          if (packedValue !== void 0)
            return packedValue;
          throw new Error("Unknown token " + token);
      }
    default:
      if (isNaN(token)) {
        let error = new Error("Unexpected end of CBOR data");
        error.incomplete = true;
        throw error;
      }
      throw new Error("Unknown CBOR token " + token);
  }
}
var validName = /^[a-zA-Z_$][a-zA-Z\d_$]*$/;
function createStructureReader(structure) {
  if (!structure) throw new Error("Structure is required in record definition");
  function readObject() {
    let length = src[position++];
    length = length & 31;
    if (length > 23) {
      switch (length) {
        case 24:
          length = src[position++];
          break;
        case 25:
          length = dataView.getUint16(position);
          position += 2;
          break;
        case 26:
          length = dataView.getUint32(position);
          position += 4;
          break;
        default:
          throw new Error("Expected array header, but got " + src[position - 1]);
      }
    }
    let compiledReader = this.compiledReader;
    while (compiledReader) {
      if (compiledReader.propertyCount === length)
        return compiledReader(read);
      compiledReader = compiledReader.next;
    }
    if (this.slowReads++ >= inlineObjectReadThreshold) {
      let array = this.length == length ? this : this.slice(0, length);
      compiledReader = currentDecoder.keyMap ? new Function("r", "return {" + array.map((k) => currentDecoder.decodeKey(k)).map((k) => validName.test(k) ? safeKey(k) + ":r()" : "[" + JSON.stringify(k) + "]:r()").join(",") + "}") : new Function("r", "return {" + array.map((key) => validName.test(key) ? safeKey(key) + ":r()" : "[" + JSON.stringify(key) + "]:r()").join(",") + "}");
      if (this.compiledReader)
        compiledReader.next = this.compiledReader;
      compiledReader.propertyCount = length;
      this.compiledReader = compiledReader;
      return compiledReader(read);
    }
    let object = {};
    if (currentDecoder.keyMap) for (let i = 0; i < length; i++) object[safeKey(currentDecoder.decodeKey(this[i]))] = read();
    else for (let i = 0; i < length; i++) {
      object[safeKey(this[i])] = read();
    }
    return object;
  }
  structure.slowReads = 0;
  return readObject;
}
function safeKey(key) {
  if (typeof key === "string") return key === "__proto__" ? "__proto_" : key;
  if (typeof key === "number" || typeof key === "boolean" || typeof key === "bigint") return key.toString();
  if (key == null) return key + "";
  throw new Error("Invalid property name type " + typeof key);
}
var readFixedString = readStringJS;
function readStringJS(length) {
  let result;
  if (length < 16) {
    if (result = shortStringInJS(length))
      return result;
  }
  if (length > 64 && decoder)
    return decoder.decode(src.subarray(position, position += length));
  const end = position + length;
  const units = [];
  result = "";
  while (position < end) {
    const byte1 = src[position++];
    if ((byte1 & 128) === 0) {
      units.push(byte1);
    } else if ((byte1 & 224) === 192) {
      const byte2 = src[position++] & 63;
      units.push((byte1 & 31) << 6 | byte2);
    } else if ((byte1 & 240) === 224) {
      const byte2 = src[position++] & 63;
      const byte3 = src[position++] & 63;
      units.push((byte1 & 31) << 12 | byte2 << 6 | byte3);
    } else if ((byte1 & 248) === 240) {
      const byte2 = src[position++] & 63;
      const byte3 = src[position++] & 63;
      const byte4 = src[position++] & 63;
      let unit = (byte1 & 7) << 18 | byte2 << 12 | byte3 << 6 | byte4;
      if (unit > 65535) {
        unit -= 65536;
        units.push(unit >>> 10 & 1023 | 55296);
        unit = 56320 | unit & 1023;
      }
      units.push(unit);
    } else {
      units.push(byte1);
    }
    if (units.length >= 4096) {
      result += fromCharCode.apply(String, units);
      units.length = 0;
    }
  }
  if (units.length > 0) {
    result += fromCharCode.apply(String, units);
  }
  return result;
}
var fromCharCode = String.fromCharCode;
function longStringInJS(length) {
  let start = position;
  let bytes = new Array(length);
  for (let i = 0; i < length; i++) {
    const byte = src[position++];
    if ((byte & 128) > 0) {
      position = start;
      return;
    }
    bytes[i] = byte;
  }
  return fromCharCode.apply(String, bytes);
}
function shortStringInJS(length) {
  if (length < 4) {
    if (length < 2) {
      if (length === 0)
        return "";
      else {
        let a = src[position++];
        if ((a & 128) > 1) {
          position -= 1;
          return;
        }
        return fromCharCode(a);
      }
    } else {
      let a = src[position++];
      let b = src[position++];
      if ((a & 128) > 0 || (b & 128) > 0) {
        position -= 2;
        return;
      }
      if (length < 3)
        return fromCharCode(a, b);
      let c = src[position++];
      if ((c & 128) > 0) {
        position -= 3;
        return;
      }
      return fromCharCode(a, b, c);
    }
  } else {
    let a = src[position++];
    let b = src[position++];
    let c = src[position++];
    let d = src[position++];
    if ((a & 128) > 0 || (b & 128) > 0 || (c & 128) > 0 || (d & 128) > 0) {
      position -= 4;
      return;
    }
    if (length < 6) {
      if (length === 4)
        return fromCharCode(a, b, c, d);
      else {
        let e = src[position++];
        if ((e & 128) > 0) {
          position -= 5;
          return;
        }
        return fromCharCode(a, b, c, d, e);
      }
    } else if (length < 8) {
      let e = src[position++];
      let f = src[position++];
      if ((e & 128) > 0 || (f & 128) > 0) {
        position -= 6;
        return;
      }
      if (length < 7)
        return fromCharCode(a, b, c, d, e, f);
      let g = src[position++];
      if ((g & 128) > 0) {
        position -= 7;
        return;
      }
      return fromCharCode(a, b, c, d, e, f, g);
    } else {
      let e = src[position++];
      let f = src[position++];
      let g = src[position++];
      let h = src[position++];
      if ((e & 128) > 0 || (f & 128) > 0 || (g & 128) > 0 || (h & 128) > 0) {
        position -= 8;
        return;
      }
      if (length < 10) {
        if (length === 8)
          return fromCharCode(a, b, c, d, e, f, g, h);
        else {
          let i = src[position++];
          if ((i & 128) > 0) {
            position -= 9;
            return;
          }
          return fromCharCode(a, b, c, d, e, f, g, h, i);
        }
      } else if (length < 12) {
        let i = src[position++];
        let j = src[position++];
        if ((i & 128) > 0 || (j & 128) > 0) {
          position -= 10;
          return;
        }
        if (length < 11)
          return fromCharCode(a, b, c, d, e, f, g, h, i, j);
        let k = src[position++];
        if ((k & 128) > 0) {
          position -= 11;
          return;
        }
        return fromCharCode(a, b, c, d, e, f, g, h, i, j, k);
      } else {
        let i = src[position++];
        let j = src[position++];
        let k = src[position++];
        let l = src[position++];
        if ((i & 128) > 0 || (j & 128) > 0 || (k & 128) > 0 || (l & 128) > 0) {
          position -= 12;
          return;
        }
        if (length < 14) {
          if (length === 12)
            return fromCharCode(a, b, c, d, e, f, g, h, i, j, k, l);
          else {
            let m = src[position++];
            if ((m & 128) > 0) {
              position -= 13;
              return;
            }
            return fromCharCode(a, b, c, d, e, f, g, h, i, j, k, l, m);
          }
        } else {
          let m = src[position++];
          let n = src[position++];
          if ((m & 128) > 0 || (n & 128) > 0) {
            position -= 14;
            return;
          }
          if (length < 15)
            return fromCharCode(a, b, c, d, e, f, g, h, i, j, k, l, m, n);
          let o = src[position++];
          if ((o & 128) > 0) {
            position -= 15;
            return;
          }
          return fromCharCode(a, b, c, d, e, f, g, h, i, j, k, l, m, n, o);
        }
      }
    }
  }
}
function readBin(length) {
  return currentDecoder.copyBuffers ? (
    // specifically use the copying slice (not the node one)
    Uint8Array.prototype.slice.call(src, position, position += length)
  ) : src.subarray(position, position += length);
}
var f32Array = new Float32Array(1);
var u8Array = new Uint8Array(f32Array.buffer, 0, 4);
function getFloat16() {
  let byte0 = src[position++];
  let byte1 = src[position++];
  let exponent = (byte0 & 127) >> 2;
  if (exponent === 31) {
    if (byte1 || byte0 & 3)
      return NaN;
    return byte0 & 128 ? -Infinity : Infinity;
  }
  if (exponent === 0) {
    let abs = ((byte0 & 3) << 8 | byte1) / (1 << 24);
    return byte0 & 128 ? -abs : abs;
  }
  u8Array[3] = byte0 & 128 | // sign bit
  (exponent >> 1) + 56;
  u8Array[2] = (byte0 & 7) << 5 | // last exponent bit and first two mantissa bits
  byte1 >> 3;
  u8Array[1] = byte1 << 5;
  u8Array[0] = 0;
  return f32Array[0];
}
var keyCache = new Array(4096);
var Tag = class {
  constructor(value, tag) {
    this.value = value;
    this.tag = tag;
  }
};
currentExtensions[0] = (dateString) => {
  return new Date(dateString);
};
currentExtensions[1] = (epochSec) => {
  return new Date(Math.round(epochSec * 1e3));
};
currentExtensions[2] = (buffer) => {
  let value = BigInt(0);
  for (let i = 0, l = buffer.byteLength; i < l; i++) {
    value = BigInt(buffer[i]) + (value << BigInt(8));
  }
  return value;
};
currentExtensions[3] = (buffer) => {
  return BigInt(-1) - currentExtensions[2](buffer);
};
currentExtensions[4] = (fraction) => {
  return +(fraction[1] + "e" + fraction[0]);
};
currentExtensions[5] = (fraction) => {
  return fraction[1] * Math.exp(fraction[0] * Math.log(2));
};
var recordDefinition = (id, structure) => {
  id = id - 57344;
  let existingStructure = currentStructures[id];
  if (existingStructure && existingStructure.isShared) {
    (currentStructures.restoreStructures || (currentStructures.restoreStructures = []))[id] = existingStructure;
  }
  currentStructures[id] = structure;
  structure.read = createStructureReader(structure);
};
currentExtensions[LEGACY_RECORD_INLINE_ID] = (data) => {
  let length = data.length;
  let structure = data[1];
  recordDefinition(data[0], structure);
  let object = {};
  for (let i = 2; i < length; i++) {
    let key = structure[i - 2];
    object[safeKey(key)] = data[i];
  }
  return object;
};
currentExtensions[14] = (value) => {
  if (bundledStrings)
    return bundledStrings[0].slice(bundledStrings.position0, bundledStrings.position0 += value);
  return new Tag(value, 14);
};
currentExtensions[15] = (value) => {
  if (bundledStrings)
    return bundledStrings[1].slice(bundledStrings.position1, bundledStrings.position1 += value);
  return new Tag(value, 15);
};
var glbl = { Error, RegExp };
currentExtensions[27] = (data) => {
  return (glbl[data[0]] || Error)(data[1], data[2]);
};
var packedTable = (read3) => {
  if (src[position++] != 132) {
    let error = new Error("Packed values structure must be followed by a 4 element array");
    if (src.length < position)
      error.incomplete = true;
    throw error;
  }
  let newPackedValues = read3();
  if (!newPackedValues || !newPackedValues.length) {
    let error = new Error("Packed values structure must be followed by a 4 element array");
    error.incomplete = true;
    throw error;
  }
  packedValues = packedValues ? newPackedValues.concat(packedValues.slice(newPackedValues.length)) : newPackedValues;
  packedValues.prefixes = read3();
  packedValues.suffixes = read3();
  return read3();
};
packedTable.handlesRead = true;
currentExtensions[51] = packedTable;
currentExtensions[PACKED_REFERENCE_TAG_ID] = (data) => {
  if (!packedValues) {
    if (currentDecoder.getShared)
      loadShared();
    else
      return new Tag(data, PACKED_REFERENCE_TAG_ID);
  }
  if (typeof data == "number")
    return packedValues[16 + (data >= 0 ? 2 * data : -2 * data - 1)];
  let error = new Error("No support for non-integer packed references yet");
  if (data === void 0)
    error.incomplete = true;
  throw error;
};
currentExtensions[28] = (read3) => {
  if (!referenceMap) {
    referenceMap = /* @__PURE__ */ new Map();
    referenceMap.id = 0;
  }
  let id = referenceMap.id++;
  let startingPosition = position;
  let token = src[position];
  let target3;
  if (token >> 5 == 4)
    target3 = [];
  else
    target3 = {};
  let refEntry = { target: target3 };
  referenceMap.set(id, refEntry);
  let targetProperties = read3();
  if (refEntry.used) {
    if (Object.getPrototypeOf(target3) !== Object.getPrototypeOf(targetProperties)) {
      position = startingPosition;
      target3 = targetProperties;
      referenceMap.set(id, { target: target3 });
      targetProperties = read3();
    }
    return Object.assign(target3, targetProperties);
  }
  refEntry.target = targetProperties;
  return targetProperties;
};
currentExtensions[28].handlesRead = true;
currentExtensions[29] = (id) => {
  let refEntry = referenceMap.get(id);
  refEntry.used = true;
  return refEntry.target;
};
currentExtensions[258] = (array) => new Set(array);
(currentExtensions[259] = (read3) => {
  if (currentDecoder.mapsAsObjects) {
    currentDecoder.mapsAsObjects = false;
    restoreMapsAsObject = true;
  }
  return read3();
}).handlesRead = true;
function combine(a, b) {
  if (typeof a === "string")
    return a + b;
  if (a instanceof Array)
    return a.concat(b);
  return Object.assign({}, a, b);
}
function getPackedValues() {
  if (!packedValues) {
    if (currentDecoder.getShared)
      loadShared();
    else
      throw new Error("No packed values available");
  }
  return packedValues;
}
var SHARED_DATA_TAG_ID = 1399353956;
currentExtensionRanges.push((tag, input) => {
  if (tag >= 225 && tag <= 255)
    return combine(getPackedValues().prefixes[tag - 224], input);
  if (tag >= 28704 && tag <= 32767)
    return combine(getPackedValues().prefixes[tag - 28672], input);
  if (tag >= 1879052288 && tag <= 2147483647)
    return combine(getPackedValues().prefixes[tag - 1879048192], input);
  if (tag >= 216 && tag <= 223)
    return combine(input, getPackedValues().suffixes[tag - 216]);
  if (tag >= 27647 && tag <= 28671)
    return combine(input, getPackedValues().suffixes[tag - 27639]);
  if (tag >= 1811940352 && tag <= 1879048191)
    return combine(input, getPackedValues().suffixes[tag - 1811939328]);
  if (tag == SHARED_DATA_TAG_ID) {
    return {
      packedValues,
      structures: currentStructures.slice(0),
      version: input
    };
  }
  if (tag == 55799)
    return input;
});
var isLittleEndianMachine = new Uint8Array(new Uint16Array([1]).buffer)[0] == 1;
var typedArrays = [
  Uint8Array,
  Uint8ClampedArray,
  Uint16Array,
  Uint32Array,
  typeof BigUint64Array == "undefined" ? { name: "BigUint64Array" } : BigUint64Array,
  Int8Array,
  Int16Array,
  Int32Array,
  typeof BigInt64Array == "undefined" ? { name: "BigInt64Array" } : BigInt64Array,
  Float32Array,
  Float64Array
];
var typedArrayTags = [64, 68, 69, 70, 71, 72, 77, 78, 79, 85, 86];
for (let i = 0; i < typedArrays.length; i++) {
  registerTypedArray(typedArrays[i], typedArrayTags[i]);
}
function registerTypedArray(TypedArray, tag) {
  let dvMethod = "get" + TypedArray.name.slice(0, -5);
  let bytesPerElement;
  if (typeof TypedArray === "function")
    bytesPerElement = TypedArray.BYTES_PER_ELEMENT;
  else
    TypedArray = null;
  for (let littleEndian = 0; littleEndian < 2; littleEndian++) {
    if (!littleEndian && bytesPerElement == 1)
      continue;
    let sizeShift = bytesPerElement == 2 ? 1 : bytesPerElement == 4 ? 2 : bytesPerElement == 8 ? 3 : 0;
    currentExtensions[littleEndian ? tag : tag - 4] = bytesPerElement == 1 || littleEndian == isLittleEndianMachine ? (buffer) => {
      if (!TypedArray)
        throw new Error("Could not find typed array for code " + tag);
      if (!currentDecoder.copyBuffers) {
        if (bytesPerElement === 1 || bytesPerElement === 2 && !(buffer.byteOffset & 1) || bytesPerElement === 4 && !(buffer.byteOffset & 3) || bytesPerElement === 8 && !(buffer.byteOffset & 7))
          return new TypedArray(buffer.buffer, buffer.byteOffset, buffer.byteLength >> sizeShift);
      }
      return new TypedArray(Uint8Array.prototype.slice.call(buffer, 0).buffer);
    } : (buffer) => {
      if (!TypedArray)
        throw new Error("Could not find typed array for code " + tag);
      let dv = new DataView(buffer.buffer, buffer.byteOffset, buffer.byteLength);
      let elements = buffer.length >> sizeShift;
      let ta = new TypedArray(elements);
      let method = dv[dvMethod];
      for (let i = 0; i < elements; i++) {
        ta[i] = method.call(dv, i << sizeShift, littleEndian);
      }
      return ta;
    };
  }
}
function readBundleExt() {
  let length = readJustLength();
  let bundlePosition = position + read();
  for (let i = 2; i < length; i++) {
    let bundleLength = readJustLength();
    position += bundleLength;
  }
  let dataPosition = position;
  position = bundlePosition;
  bundledStrings = [readStringJS(readJustLength()), readStringJS(readJustLength())];
  bundledStrings.position0 = 0;
  bundledStrings.position1 = 0;
  bundledStrings.postBundlePosition = position;
  position = dataPosition;
  return read();
}
function readJustLength() {
  let token = src[position++] & 31;
  if (token > 23) {
    switch (token) {
      case 24:
        token = src[position++];
        break;
      case 25:
        token = dataView.getUint16(position);
        position += 2;
        break;
      case 26:
        token = dataView.getUint32(position);
        position += 4;
        break;
    }
  }
  return token;
}
function loadShared() {
  if (currentDecoder.getShared) {
    let sharedData = saveState(() => {
      src = null;
      return currentDecoder.getShared();
    }) || {};
    let updatedStructures = sharedData.structures || [];
    currentDecoder.sharedVersion = sharedData.version;
    packedValues = currentDecoder.sharedValues = sharedData.packedValues;
    if (currentStructures === true)
      currentDecoder.structures = currentStructures = updatedStructures;
    else
      currentStructures.splice.apply(currentStructures, [0, updatedStructures.length].concat(updatedStructures));
  }
}
function saveState(callback) {
  let savedSrcEnd = srcEnd;
  let savedPosition = position;
  let savedStringPosition = stringPosition;
  let savedSrcStringStart = srcStringStart;
  let savedSrcStringEnd = srcStringEnd;
  let savedSrcString = srcString;
  let savedStrings = strings;
  let savedReferenceMap = referenceMap;
  let savedBundledStrings = bundledStrings;
  let savedSrc = new Uint8Array(src.slice(0, srcEnd));
  let savedStructures = currentStructures;
  let savedDecoder = currentDecoder;
  let savedSequentialMode = sequentialMode;
  let value = callback();
  srcEnd = savedSrcEnd;
  position = savedPosition;
  stringPosition = savedStringPosition;
  srcStringStart = savedSrcStringStart;
  srcStringEnd = savedSrcStringEnd;
  srcString = savedSrcString;
  strings = savedStrings;
  referenceMap = savedReferenceMap;
  bundledStrings = savedBundledStrings;
  src = savedSrc;
  sequentialMode = savedSequentialMode;
  currentStructures = savedStructures;
  currentDecoder = savedDecoder;
  dataView = new DataView(src.buffer, src.byteOffset, src.byteLength);
  return value;
}
function clearSource() {
  src = null;
  referenceMap = null;
  currentStructures = null;
}
var mult10 = new Array(147);
for (let i = 0; i < 256; i++) {
  mult10[i] = +("1e" + Math.floor(45.15 - i * 0.30103));
}
var defaultDecoder = new Decoder({ useRecords: false });
var decode = defaultDecoder.decode;
var decodeMultiple = defaultDecoder.decodeMultiple;
var FLOAT32_OPTIONS = {
  NEVER: 0,
  ALWAYS: 1,
  DECIMAL_ROUND: 3,
  DECIMAL_FIT: 4
};

// node_modules/cbor-x/encode.js
var textEncoder;
try {
  textEncoder = new TextEncoder();
} catch (error) {
}
var extensions;
var extensionClasses;
var Buffer2 = typeof globalThis === "object" && globalThis.Buffer;
var hasNodeBuffer = typeof Buffer2 !== "undefined";
var ByteArrayAllocate = hasNodeBuffer ? Buffer2.allocUnsafeSlow : Uint8Array;
var ByteArray = hasNodeBuffer ? Buffer2 : Uint8Array;
var MAX_STRUCTURES = 256;
var MAX_BUFFER_SIZE = hasNodeBuffer ? 4294967296 : 2144337920;
var throwOnIterable;
var target;
var targetView;
var position2 = 0;
var safeEnd;
var bundledStrings2 = null;
var MAX_BUNDLE_SIZE = 61440;
var hasNonLatin = /[\u0080-\uFFFF]/;
var RECORD_SYMBOL = Symbol("record-id");
var Encoder = class extends Decoder {
  constructor(options) {
    super(options);
    this.offset = 0;
    let typeBuffer;
    let start;
    let sharedStructures;
    let hasSharedUpdate;
    let structures;
    let referenceMap3;
    options = options || {};
    let encodeUtf8 = ByteArray.prototype.utf8Write ? function(string, position5, maxBytes) {
      return target.utf8Write(string, position5, maxBytes);
    } : textEncoder && textEncoder.encodeInto ? function(string, position5) {
      return textEncoder.encodeInto(string, target.subarray(position5)).written;
    } : false;
    let encoder = this;
    let hasSharedStructures = options.structures || options.saveStructures;
    let maxSharedStructures = options.maxSharedStructures;
    if (maxSharedStructures == null)
      maxSharedStructures = hasSharedStructures ? 128 : 0;
    if (maxSharedStructures > 8190)
      throw new Error("Maximum maxSharedStructure is 8190");
    let isSequential = options.sequential;
    if (isSequential) {
      maxSharedStructures = 0;
    }
    if (!this.structures)
      this.structures = [];
    if (this.saveStructures)
      this.saveShared = this.saveStructures;
    let samplingPackedValues, packedObjectMap2, sharedValues = options.sharedValues;
    let sharedPackedObjectMap2;
    if (sharedValues) {
      sharedPackedObjectMap2 = /* @__PURE__ */ Object.create(null);
      for (let i = 0, l = sharedValues.length; i < l; i++) {
        sharedPackedObjectMap2[sharedValues[i]] = i;
      }
    }
    let recordIdsToRemove = [];
    let transitionsCount = 0;
    let serializationsSinceTransitionRebuild = 0;
    this.mapEncode = function(value, encodeOptions) {
      if (this._keyMap && !this._mapped) {
        switch (value.constructor.name) {
          case "Array":
            value = value.map((r) => this.encodeKeys(r));
            break;
        }
      }
      return this.encode(value, encodeOptions);
    };
    this.encode = function(value, encodeOptions) {
      if (!target) {
        target = new ByteArrayAllocate(8192);
        targetView = new DataView(target.buffer, 0, 8192);
        position2 = 0;
      }
      safeEnd = target.length - 10;
      if (safeEnd - position2 < 2048) {
        target = new ByteArrayAllocate(target.length);
        targetView = new DataView(target.buffer, 0, target.length);
        safeEnd = target.length - 10;
        position2 = 0;
      } else if (encodeOptions === REUSE_BUFFER_MODE)
        position2 = position2 + 7 & 2147483640;
      start = position2;
      if (encoder.useSelfDescribedHeader) {
        targetView.setUint32(position2, 3654940416);
        position2 += 3;
      }
      referenceMap3 = encoder.structuredClone ? /* @__PURE__ */ new Map() : null;
      if (encoder.bundleStrings && typeof value !== "string") {
        bundledStrings2 = [];
        bundledStrings2.size = Infinity;
      } else
        bundledStrings2 = null;
      sharedStructures = encoder.structures;
      if (sharedStructures) {
        if (sharedStructures.uninitialized) {
          let sharedData = encoder.getShared() || {};
          encoder.structures = sharedStructures = sharedData.structures || [];
          encoder.sharedVersion = sharedData.version;
          let sharedValues2 = encoder.sharedValues = sharedData.packedValues;
          if (sharedValues2) {
            sharedPackedObjectMap2 = {};
            for (let i = 0, l = sharedValues2.length; i < l; i++)
              sharedPackedObjectMap2[sharedValues2[i]] = i;
          }
        }
        let sharedStructuresLength = sharedStructures.length;
        if (sharedStructuresLength > maxSharedStructures && !isSequential)
          sharedStructuresLength = maxSharedStructures;
        if (!sharedStructures.transitions) {
          sharedStructures.transitions = /* @__PURE__ */ Object.create(null);
          for (let i = 0; i < sharedStructuresLength; i++) {
            let keys = sharedStructures[i];
            if (!keys)
              continue;
            let nextTransition, transition = sharedStructures.transitions;
            for (let j = 0, l = keys.length; j < l; j++) {
              if (transition[RECORD_SYMBOL] === void 0)
                transition[RECORD_SYMBOL] = i;
              let key = keys[j];
              nextTransition = transition[key];
              if (!nextTransition) {
                nextTransition = transition[key] = /* @__PURE__ */ Object.create(null);
              }
              transition = nextTransition;
            }
            transition[RECORD_SYMBOL] = i | 1048576;
          }
        }
        if (!isSequential)
          sharedStructures.nextId = sharedStructuresLength;
      }
      if (hasSharedUpdate)
        hasSharedUpdate = false;
      structures = sharedStructures || [];
      packedObjectMap2 = sharedPackedObjectMap2;
      if (options.pack) {
        let packedValues3 = /* @__PURE__ */ new Map();
        packedValues3.values = [];
        packedValues3.encoder = encoder;
        packedValues3.maxValues = options.maxPrivatePackedValues || (sharedPackedObjectMap2 ? 16 : Infinity);
        packedValues3.objectMap = sharedPackedObjectMap2 || false;
        packedValues3.samplingPackedValues = samplingPackedValues;
        findRepetitiveStrings(value, packedValues3);
        if (packedValues3.values.length > 0) {
          target[position2++] = 216;
          target[position2++] = 51;
          writeArrayHeader(4);
          let valuesArray = packedValues3.values;
          encode3(valuesArray);
          writeArrayHeader(0);
          writeArrayHeader(0);
          packedObjectMap2 = Object.create(sharedPackedObjectMap2 || null);
          for (let i = 0, l = valuesArray.length; i < l; i++) {
            packedObjectMap2[valuesArray[i]] = i;
          }
        }
      }
      throwOnIterable = encodeOptions & THROW_ON_ITERABLE;
      try {
        if (throwOnIterable)
          return;
        encode3(value);
        if (bundledStrings2) {
          writeBundles(start, encode3);
        }
        encoder.offset = position2;
        if (referenceMap3 && referenceMap3.idsToInsert) {
          position2 += referenceMap3.idsToInsert.length * 2;
          if (position2 > safeEnd)
            makeRoom(position2);
          encoder.offset = position2;
          let serialized = insertIds(target.subarray(start, position2), referenceMap3.idsToInsert);
          referenceMap3 = null;
          return serialized;
        }
        if (encodeOptions & REUSE_BUFFER_MODE) {
          target.start = start;
          target.end = position2;
          return target;
        }
        return target.subarray(start, position2);
      } finally {
        if (sharedStructures) {
          if (serializationsSinceTransitionRebuild < 10)
            serializationsSinceTransitionRebuild++;
          if (sharedStructures.length > maxSharedStructures)
            sharedStructures.length = maxSharedStructures;
          if (transitionsCount > 1e4) {
            sharedStructures.transitions = null;
            serializationsSinceTransitionRebuild = 0;
            transitionsCount = 0;
            if (recordIdsToRemove.length > 0)
              recordIdsToRemove = [];
          } else if (recordIdsToRemove.length > 0 && !isSequential) {
            for (let i = 0, l = recordIdsToRemove.length; i < l; i++) {
              recordIdsToRemove[i][RECORD_SYMBOL] = void 0;
            }
            recordIdsToRemove = [];
          }
        }
        if (hasSharedUpdate && encoder.saveShared) {
          if (encoder.structures.length > maxSharedStructures) {
            encoder.structures = encoder.structures.slice(0, maxSharedStructures);
          }
          let returnBuffer = target.subarray(start, position2);
          if (encoder.updateSharedData() === false)
            return encoder.encode(value);
          return returnBuffer;
        }
        if (encodeOptions & RESET_BUFFER_MODE)
          position2 = start;
      }
    };
    this.findCommonStringsToPack = () => {
      samplingPackedValues = /* @__PURE__ */ new Map();
      if (!sharedPackedObjectMap2)
        sharedPackedObjectMap2 = /* @__PURE__ */ Object.create(null);
      return (options2) => {
        let threshold = options2 && options2.threshold || 4;
        let position5 = this.pack ? options2.maxPrivatePackedValues || 16 : 0;
        if (!sharedValues)
          sharedValues = this.sharedValues = [];
        for (let [key, status] of samplingPackedValues) {
          if (status.count > threshold) {
            sharedPackedObjectMap2[key] = position5++;
            sharedValues.push(key);
            hasSharedUpdate = true;
          }
        }
        while (this.saveShared && this.updateSharedData() === false) {
        }
        samplingPackedValues = null;
      };
    };
    const encode3 = (value) => {
      if (position2 > safeEnd)
        target = makeRoom(position2);
      var type = typeof value;
      var length;
      if (type === "string") {
        if (packedObjectMap2) {
          let packedPosition = packedObjectMap2[value];
          if (packedPosition >= 0) {
            if (packedPosition < 16)
              target[position2++] = packedPosition + 224;
            else {
              target[position2++] = 198;
              if (packedPosition & 1)
                encode3(15 - packedPosition >> 1);
              else
                encode3(packedPosition - 16 >> 1);
            }
            return;
          } else if (samplingPackedValues && !options.pack) {
            let status = samplingPackedValues.get(value);
            if (status)
              status.count++;
            else
              samplingPackedValues.set(value, {
                count: 1
              });
          }
        }
        let strLength = value.length;
        if (bundledStrings2 && strLength >= 4 && strLength < 1024) {
          if ((bundledStrings2.size += strLength) > MAX_BUNDLE_SIZE) {
            let extStart;
            let maxBytes2 = (bundledStrings2[0] ? bundledStrings2[0].length * 3 + bundledStrings2[1].length : 0) + 10;
            if (position2 + maxBytes2 > safeEnd)
              target = makeRoom(position2 + maxBytes2);
            target[position2++] = 217;
            target[position2++] = 223;
            target[position2++] = 249;
            target[position2++] = bundledStrings2.position ? 132 : 130;
            target[position2++] = 26;
            extStart = position2 - start;
            position2 += 4;
            if (bundledStrings2.position) {
              writeBundles(start, encode3);
            }
            bundledStrings2 = ["", ""];
            bundledStrings2.size = 0;
            bundledStrings2.position = extStart;
          }
          let twoByte = hasNonLatin.test(value);
          bundledStrings2[twoByte ? 0 : 1] += value;
          target[position2++] = twoByte ? 206 : 207;
          encode3(strLength);
          return;
        }
        let headerSize;
        if (strLength < 32) {
          headerSize = 1;
        } else if (strLength < 256) {
          headerSize = 2;
        } else if (strLength < 65536) {
          headerSize = 3;
        } else {
          headerSize = 5;
        }
        let maxBytes = strLength * 3;
        if (position2 + maxBytes > safeEnd)
          target = makeRoom(position2 + maxBytes);
        if (strLength < 64 || !encodeUtf8) {
          let i, c1, c2, strPosition = position2 + headerSize;
          for (i = 0; i < strLength; i++) {
            c1 = value.charCodeAt(i);
            if (c1 < 128) {
              target[strPosition++] = c1;
            } else if (c1 < 2048) {
              target[strPosition++] = c1 >> 6 | 192;
              target[strPosition++] = c1 & 63 | 128;
            } else if ((c1 & 64512) === 55296 && ((c2 = value.charCodeAt(i + 1)) & 64512) === 56320) {
              c1 = 65536 + ((c1 & 1023) << 10) + (c2 & 1023);
              i++;
              target[strPosition++] = c1 >> 18 | 240;
              target[strPosition++] = c1 >> 12 & 63 | 128;
              target[strPosition++] = c1 >> 6 & 63 | 128;
              target[strPosition++] = c1 & 63 | 128;
            } else {
              target[strPosition++] = c1 >> 12 | 224;
              target[strPosition++] = c1 >> 6 & 63 | 128;
              target[strPosition++] = c1 & 63 | 128;
            }
          }
          length = strPosition - position2 - headerSize;
        } else {
          length = encodeUtf8(value, position2 + headerSize, maxBytes);
        }
        if (length < 24) {
          target[position2++] = 96 | length;
        } else if (length < 256) {
          if (headerSize < 2) {
            target.copyWithin(position2 + 2, position2 + 1, position2 + 1 + length);
          }
          target[position2++] = 120;
          target[position2++] = length;
        } else if (length < 65536) {
          if (headerSize < 3) {
            target.copyWithin(position2 + 3, position2 + 2, position2 + 2 + length);
          }
          target[position2++] = 121;
          target[position2++] = length >> 8;
          target[position2++] = length & 255;
        } else {
          if (headerSize < 5) {
            target.copyWithin(position2 + 5, position2 + 3, position2 + 3 + length);
          }
          target[position2++] = 122;
          targetView.setUint32(position2, length);
          position2 += 4;
        }
        position2 += length;
      } else if (type === "number") {
        if (!this.alwaysUseFloat && value >>> 0 === value) {
          if (value < 24) {
            target[position2++] = value;
          } else if (value < 256) {
            target[position2++] = 24;
            target[position2++] = value;
          } else if (value < 65536) {
            target[position2++] = 25;
            target[position2++] = value >> 8;
            target[position2++] = value & 255;
          } else {
            target[position2++] = 26;
            targetView.setUint32(position2, value);
            position2 += 4;
          }
        } else if (!this.alwaysUseFloat && value >> 0 === value) {
          if (value >= -24) {
            target[position2++] = 31 - value;
          } else if (value >= -256) {
            target[position2++] = 56;
            target[position2++] = ~value;
          } else if (value >= -65536) {
            target[position2++] = 57;
            targetView.setUint16(position2, ~value);
            position2 += 2;
          } else {
            target[position2++] = 58;
            targetView.setUint32(position2, ~value);
            position2 += 4;
          }
        } else {
          let useFloat32;
          if ((useFloat32 = this.useFloat32) > 0 && value < 4294967296 && value >= -2147483648) {
            target[position2++] = 250;
            targetView.setFloat32(position2, value);
            let xShifted;
            if (useFloat32 < 4 || // this checks for rounding of numbers that were encoded in 32-bit float to nearest significant decimal digit that could be preserved
            (xShifted = value * mult10[(target[position2] & 127) << 1 | target[position2 + 1] >> 7]) >> 0 === xShifted) {
              position2 += 4;
              return;
            } else
              position2--;
          }
          target[position2++] = 251;
          targetView.setFloat64(position2, value);
          position2 += 8;
        }
      } else if (type === "object") {
        if (!value)
          target[position2++] = 246;
        else {
          if (referenceMap3) {
            let referee = referenceMap3.get(value);
            if (referee) {
              target[position2++] = 216;
              target[position2++] = 29;
              target[position2++] = 25;
              if (!referee.references) {
                let idsToInsert = referenceMap3.idsToInsert || (referenceMap3.idsToInsert = []);
                referee.references = [];
                idsToInsert.push(referee);
              }
              referee.references.push(position2 - start);
              position2 += 2;
              return;
            } else
              referenceMap3.set(value, { offset: position2 - start });
          }
          let constructor = value.constructor;
          if (constructor === Object) {
            writeObject(value);
          } else if (constructor === Array) {
            length = value.length;
            if (length < 24) {
              target[position2++] = 128 | length;
            } else {
              writeArrayHeader(length);
            }
            for (let i = 0; i < length; i++) {
              encode3(value[i]);
            }
          } else if (constructor === Map) {
            if (this.mapsAsObjects ? this.useTag259ForMaps !== false : this.useTag259ForMaps) {
              target[position2++] = 217;
              target[position2++] = 1;
              target[position2++] = 3;
            }
            length = value.size;
            if (length < 24) {
              target[position2++] = 160 | length;
            } else if (length < 256) {
              target[position2++] = 184;
              target[position2++] = length;
            } else if (length < 65536) {
              target[position2++] = 185;
              target[position2++] = length >> 8;
              target[position2++] = length & 255;
            } else {
              target[position2++] = 186;
              targetView.setUint32(position2, length);
              position2 += 4;
            }
            if (encoder.keyMap) {
              for (let [key, entryValue] of value) {
                encode3(encoder.encodeKey(key));
                encode3(entryValue);
              }
            } else {
              for (let [key, entryValue] of value) {
                encode3(key);
                encode3(entryValue);
              }
            }
          } else {
            for (let i = 0, l = extensions.length; i < l; i++) {
              let extensionClass = extensionClasses[i];
              if (value instanceof extensionClass) {
                let extension = extensions[i];
                let tag = extension.tag;
                if (tag == void 0)
                  tag = extension.getTag && extension.getTag.call(this, value);
                if (tag < 24) {
                  target[position2++] = 192 | tag;
                } else if (tag < 256) {
                  target[position2++] = 216;
                  target[position2++] = tag;
                } else if (tag < 65536) {
                  target[position2++] = 217;
                  target[position2++] = tag >> 8;
                  target[position2++] = tag & 255;
                } else if (tag > -1) {
                  target[position2++] = 218;
                  targetView.setUint32(position2, tag);
                  position2 += 4;
                }
                extension.encode.call(this, value, encode3, makeRoom);
                return;
              }
            }
            if (value[Symbol.iterator]) {
              if (throwOnIterable) {
                let error = new Error("Iterable should be serialized as iterator");
                error.iteratorNotHandled = true;
                throw error;
              }
              target[position2++] = 159;
              for (let entry of value) {
                encode3(entry);
              }
              target[position2++] = 255;
              return;
            }
            if (value[Symbol.asyncIterator] || isBlob(value)) {
              let error = new Error("Iterable/blob should be serialized as iterator");
              error.iteratorNotHandled = true;
              throw error;
            }
            if (this.useToJSON && value.toJSON) {
              const json = value.toJSON();
              if (json !== value)
                return encode3(json);
            }
            writeObject(value);
          }
        }
      } else if (type === "boolean") {
        target[position2++] = value ? 245 : 244;
      } else if (type === "bigint") {
        if (value < BigInt(1) << BigInt(64) && value >= 0) {
          target[position2++] = 27;
          targetView.setBigUint64(position2, value);
        } else if (value > -(BigInt(1) << BigInt(64)) && value < 0) {
          target[position2++] = 59;
          targetView.setBigUint64(position2, -value - BigInt(1));
        } else {
          if (this.largeBigIntToFloat) {
            target[position2++] = 251;
            targetView.setFloat64(position2, Number(value));
          } else {
            if (value >= BigInt(0))
              target[position2++] = 194;
            else {
              target[position2++] = 195;
              value = BigInt(-1) - value;
            }
            let bytes = [];
            while (value) {
              bytes.push(Number(value & BigInt(255)));
              value >>= BigInt(8);
            }
            writeBuffer(new Uint8Array(bytes.reverse()), makeRoom);
            return;
          }
        }
        position2 += 8;
      } else if (type === "undefined") {
        target[position2++] = 247;
      } else {
        throw new Error("Unknown type: " + type);
      }
    };
    const writeObject = this.useRecords === false ? this.variableMapSize ? (object) => {
      let keys = Object.keys(object);
      let vals = Object.values(object);
      let length = keys.length;
      if (length < 24) {
        target[position2++] = 160 | length;
      } else if (length < 256) {
        target[position2++] = 184;
        target[position2++] = length;
      } else if (length < 65536) {
        target[position2++] = 185;
        target[position2++] = length >> 8;
        target[position2++] = length & 255;
      } else {
        target[position2++] = 186;
        targetView.setUint32(position2, length);
        position2 += 4;
      }
      let key;
      if (encoder.keyMap) {
        for (let i = 0; i < length; i++) {
          encode3(encoder.encodeKey(keys[i]));
          encode3(vals[i]);
        }
      } else {
        for (let i = 0; i < length; i++) {
          encode3(keys[i]);
          encode3(vals[i]);
        }
      }
    } : (object) => {
      target[position2++] = 185;
      let objectOffset = position2 - start;
      position2 += 2;
      let size = 0;
      if (encoder.keyMap) {
        for (let key in object) if (typeof object.hasOwnProperty !== "function" || object.hasOwnProperty(key)) {
          encode3(encoder.encodeKey(key));
          encode3(object[key]);
          size++;
        }
      } else {
        for (let key in object) if (typeof object.hasOwnProperty !== "function" || object.hasOwnProperty(key)) {
          encode3(key);
          encode3(object[key]);
          size++;
        }
      }
      target[objectOffset++ + start] = size >> 8;
      target[objectOffset + start] = size & 255;
    } : (object, skipValues) => {
      let nextTransition, transition = structures.transitions || (structures.transitions = /* @__PURE__ */ Object.create(null));
      let newTransitions = 0;
      let length = 0;
      let parentRecordId;
      let keys;
      if (this.keyMap) {
        keys = Object.keys(object).map((k) => this.encodeKey(k));
        length = keys.length;
        for (let i = 0; i < length; i++) {
          let key = keys[i];
          nextTransition = transition[key];
          if (!nextTransition) {
            nextTransition = transition[key] = /* @__PURE__ */ Object.create(null);
            newTransitions++;
          }
          transition = nextTransition;
        }
      } else {
        for (let key in object) if (typeof object.hasOwnProperty !== "function" || object.hasOwnProperty(key)) {
          nextTransition = transition[key];
          if (!nextTransition) {
            if (transition[RECORD_SYMBOL] & 1048576) {
              parentRecordId = transition[RECORD_SYMBOL] & 65535;
            }
            nextTransition = transition[key] = /* @__PURE__ */ Object.create(null);
            newTransitions++;
          }
          transition = nextTransition;
          length++;
        }
      }
      let recordId = transition[RECORD_SYMBOL];
      if (recordId !== void 0) {
        recordId &= 65535;
        target[position2++] = 217;
        target[position2++] = recordId >> 8 | 224;
        target[position2++] = recordId & 255;
      } else {
        if (!keys)
          keys = transition.__keys__ || (transition.__keys__ = Object.keys(object));
        if (parentRecordId === void 0) {
          recordId = structures.nextId++;
          if (!recordId) {
            recordId = 0;
            structures.nextId = 1;
          }
          if (recordId >= MAX_STRUCTURES) {
            structures.nextId = (recordId = maxSharedStructures) + 1;
          }
        } else {
          recordId = parentRecordId;
        }
        structures[recordId] = keys;
        if (recordId < maxSharedStructures) {
          target[position2++] = 217;
          target[position2++] = recordId >> 8 | 224;
          target[position2++] = recordId & 255;
          transition = structures.transitions;
          for (let i = 0; i < length; i++) {
            if (transition[RECORD_SYMBOL] === void 0 || transition[RECORD_SYMBOL] & 1048576)
              transition[RECORD_SYMBOL] = recordId;
            transition = transition[keys[i]];
          }
          transition[RECORD_SYMBOL] = recordId | 1048576;
          hasSharedUpdate = true;
        } else {
          transition[RECORD_SYMBOL] = recordId;
          targetView.setUint32(position2, 3655335680);
          position2 += 3;
          if (newTransitions)
            transitionsCount += serializationsSinceTransitionRebuild * newTransitions;
          if (recordIdsToRemove.length >= MAX_STRUCTURES - maxSharedStructures)
            recordIdsToRemove.shift()[RECORD_SYMBOL] = void 0;
          recordIdsToRemove.push(transition);
          writeArrayHeader(length + 2);
          encode3(57344 + recordId);
          encode3(keys);
          if (skipValues) return;
          for (let key in object)
            if (typeof object.hasOwnProperty !== "function" || object.hasOwnProperty(key))
              encode3(object[key]);
          return;
        }
      }
      if (length < 24) {
        target[position2++] = 128 | length;
      } else {
        writeArrayHeader(length);
      }
      if (skipValues) return;
      for (let key in object)
        if (typeof object.hasOwnProperty !== "function" || object.hasOwnProperty(key))
          encode3(object[key]);
    };
    const makeRoom = (end) => {
      let newSize;
      if (end > 16777216) {
        if (end - start > MAX_BUFFER_SIZE)
          throw new Error("Encoded buffer would be larger than maximum buffer size");
        newSize = Math.min(
          MAX_BUFFER_SIZE,
          Math.round(Math.max((end - start) * (end > 67108864 ? 1.25 : 2), 4194304) / 4096) * 4096
        );
      } else
        newSize = (Math.max(end - start << 2, target.length - 1) >> 12) + 1 << 12;
      let newBuffer = new ByteArrayAllocate(newSize);
      targetView = new DataView(newBuffer.buffer, 0, newSize);
      if (target.copy)
        target.copy(newBuffer, 0, start, end);
      else
        newBuffer.set(target.slice(start, end));
      position2 -= start;
      start = 0;
      safeEnd = newBuffer.length - 10;
      return target = newBuffer;
    };
    let chunkThreshold = 100;
    let continuedChunkThreshold = 1e3;
    this.encodeAsIterable = function(value, options2) {
      return startEncoding(value, options2, encodeObjectAsIterable);
    };
    this.encodeAsAsyncIterable = function(value, options2) {
      return startEncoding(value, options2, encodeObjectAsAsyncIterable);
    };
    function* encodeObjectAsIterable(object, iterateProperties, finalIterable) {
      let constructor = object.constructor;
      if (constructor === Object) {
        let useRecords = encoder.useRecords !== false;
        if (useRecords)
          writeObject(object, true);
        else
          writeEntityLength(Object.keys(object).length, 160);
        for (let key in object) {
          let value = object[key];
          if (!useRecords) encode3(key);
          if (value && typeof value === "object") {
            if (iterateProperties[key])
              yield* encodeObjectAsIterable(value, iterateProperties[key]);
            else
              yield* tryEncode(value, iterateProperties, key);
          } else encode3(value);
        }
      } else if (constructor === Array) {
        let length = object.length;
        writeArrayHeader(length);
        for (let i = 0; i < length; i++) {
          let value = object[i];
          if (value && (typeof value === "object" || position2 - start > chunkThreshold)) {
            if (iterateProperties.element)
              yield* encodeObjectAsIterable(value, iterateProperties.element);
            else
              yield* tryEncode(value, iterateProperties, "element");
          } else encode3(value);
        }
      } else if (object[Symbol.iterator] && !object.buffer) {
        target[position2++] = 159;
        for (let value of object) {
          if (value && (typeof value === "object" || position2 - start > chunkThreshold)) {
            if (iterateProperties.element)
              yield* encodeObjectAsIterable(value, iterateProperties.element);
            else
              yield* tryEncode(value, iterateProperties, "element");
          } else encode3(value);
        }
        target[position2++] = 255;
      } else if (isBlob(object)) {
        writeEntityLength(object.size, 64);
        yield target.subarray(start, position2);
        yield object;
        restartEncoding();
      } else if (object[Symbol.asyncIterator]) {
        target[position2++] = 159;
        yield target.subarray(start, position2);
        yield object;
        restartEncoding();
        target[position2++] = 255;
      } else {
        encode3(object);
      }
      if (finalIterable && position2 > start) yield target.subarray(start, position2);
      else if (position2 - start > chunkThreshold) {
        yield target.subarray(start, position2);
        restartEncoding();
      }
    }
    function* tryEncode(value, iterateProperties, key) {
      let restart = position2 - start;
      try {
        encode3(value);
        if (position2 - start > chunkThreshold) {
          yield target.subarray(start, position2);
          restartEncoding();
        }
      } catch (error) {
        if (error.iteratorNotHandled) {
          iterateProperties[key] = {};
          position2 = start + restart;
          yield* encodeObjectAsIterable.call(this, value, iterateProperties[key]);
        } else throw error;
      }
    }
    function restartEncoding() {
      chunkThreshold = continuedChunkThreshold;
      encoder.encode(null, THROW_ON_ITERABLE);
    }
    function startEncoding(value, options2, encodeIterable) {
      if (options2 && options2.chunkThreshold)
        chunkThreshold = continuedChunkThreshold = options2.chunkThreshold;
      else
        chunkThreshold = 100;
      if (value && typeof value === "object") {
        encoder.encode(null, THROW_ON_ITERABLE);
        return encodeIterable(value, encoder.iterateProperties || (encoder.iterateProperties = {}), true);
      }
      return [encoder.encode(value)];
    }
    async function* encodeObjectAsAsyncIterable(value, iterateProperties) {
      for (let encodedValue of encodeObjectAsIterable(value, iterateProperties, true)) {
        let constructor = encodedValue.constructor;
        if (constructor === ByteArray || constructor === Uint8Array)
          yield encodedValue;
        else if (isBlob(encodedValue)) {
          let reader = encodedValue.stream().getReader();
          let next;
          while (!(next = await reader.read()).done) {
            yield next.value;
          }
        } else if (encodedValue[Symbol.asyncIterator]) {
          for await (let asyncValue of encodedValue) {
            restartEncoding();
            if (asyncValue)
              yield* encodeObjectAsAsyncIterable(asyncValue, iterateProperties.async || (iterateProperties.async = {}));
            else yield encoder.encode(asyncValue);
          }
        } else {
          yield encodedValue;
        }
      }
    }
  }
  useBuffer(buffer) {
    target = buffer;
    targetView = new DataView(target.buffer, target.byteOffset, target.byteLength);
    position2 = 0;
  }
  clearSharedData() {
    if (this.structures)
      this.structures = [];
    if (this.sharedValues)
      this.sharedValues = void 0;
  }
  updateSharedData() {
    let lastVersion = this.sharedVersion || 0;
    this.sharedVersion = lastVersion + 1;
    let structuresCopy = this.structures.slice(0);
    let sharedData = new SharedData(structuresCopy, this.sharedValues, this.sharedVersion);
    let saveResults = this.saveShared(
      sharedData,
      (existingShared) => (existingShared && existingShared.version || 0) == lastVersion
    );
    if (saveResults === false) {
      sharedData = this.getShared() || {};
      this.structures = sharedData.structures || [];
      this.sharedValues = sharedData.packedValues;
      this.sharedVersion = sharedData.version;
      this.structures.nextId = this.structures.length;
    } else {
      structuresCopy.forEach((structure, i) => this.structures[i] = structure);
    }
    return saveResults;
  }
};
function writeEntityLength(length, majorValue) {
  if (length < 24)
    target[position2++] = majorValue | length;
  else if (length < 256) {
    target[position2++] = majorValue | 24;
    target[position2++] = length;
  } else if (length < 65536) {
    target[position2++] = majorValue | 25;
    target[position2++] = length >> 8;
    target[position2++] = length & 255;
  } else {
    target[position2++] = majorValue | 26;
    targetView.setUint32(position2, length);
    position2 += 4;
  }
}
var SharedData = class {
  constructor(structures, values, version) {
    this.structures = structures;
    this.packedValues = values;
    this.version = version;
  }
};
function writeArrayHeader(length) {
  if (length < 24)
    target[position2++] = 128 | length;
  else if (length < 256) {
    target[position2++] = 152;
    target[position2++] = length;
  } else if (length < 65536) {
    target[position2++] = 153;
    target[position2++] = length >> 8;
    target[position2++] = length & 255;
  } else {
    target[position2++] = 154;
    targetView.setUint32(position2, length);
    position2 += 4;
  }
}
var BlobConstructor = typeof Blob === "undefined" ? function() {
} : Blob;
function isBlob(object) {
  if (object instanceof BlobConstructor)
    return true;
  let tag = object[Symbol.toStringTag];
  return tag === "Blob" || tag === "File";
}
function findRepetitiveStrings(value, packedValues3) {
  switch (typeof value) {
    case "string":
      if (value.length > 3) {
        if (packedValues3.objectMap[value] > -1 || packedValues3.values.length >= packedValues3.maxValues)
          return;
        let packedStatus = packedValues3.get(value);
        if (packedStatus) {
          if (++packedStatus.count == 2) {
            packedValues3.values.push(value);
          }
        } else {
          packedValues3.set(value, {
            count: 1
          });
          if (packedValues3.samplingPackedValues) {
            let status = packedValues3.samplingPackedValues.get(value);
            if (status)
              status.count++;
            else
              packedValues3.samplingPackedValues.set(value, {
                count: 1
              });
          }
        }
      }
      break;
    case "object":
      if (value) {
        if (value instanceof Array) {
          for (let i = 0, l = value.length; i < l; i++) {
            findRepetitiveStrings(value[i], packedValues3);
          }
        } else {
          let includeKeys = !packedValues3.encoder.useRecords;
          for (var key in value) {
            if (value.hasOwnProperty(key)) {
              if (includeKeys)
                findRepetitiveStrings(key, packedValues3);
              findRepetitiveStrings(value[key], packedValues3);
            }
          }
        }
      }
      break;
    case "function":
      console.log(value);
  }
}
var isLittleEndianMachine2 = new Uint8Array(new Uint16Array([1]).buffer)[0] == 1;
extensionClasses = [
  Date,
  Set,
  Error,
  RegExp,
  Tag,
  ArrayBuffer,
  Uint8Array,
  Uint8ClampedArray,
  Uint16Array,
  Uint32Array,
  typeof BigUint64Array == "undefined" ? function() {
  } : BigUint64Array,
  Int8Array,
  Int16Array,
  Int32Array,
  typeof BigInt64Array == "undefined" ? function() {
  } : BigInt64Array,
  Float32Array,
  Float64Array,
  SharedData
];
extensions = [
  {
    // Date
    tag: 1,
    encode(date, encode3) {
      let seconds = date.getTime() / 1e3;
      if ((this.useTimestamp32 || date.getMilliseconds() === 0) && seconds >= 0 && seconds < 4294967296) {
        target[position2++] = 26;
        targetView.setUint32(position2, seconds);
        position2 += 4;
      } else {
        target[position2++] = 251;
        targetView.setFloat64(position2, seconds);
        position2 += 8;
      }
    }
  },
  {
    // Set
    tag: 258,
    // https://github.com/input-output-hk/cbor-sets-spec/blob/master/CBOR_SETS.md
    encode(set, encode3) {
      let array = Array.from(set);
      encode3(array);
    }
  },
  {
    // Error
    tag: 27,
    // http://cbor.schmorp.de/generic-object
    encode(error, encode3) {
      encode3([error.name, error.message]);
    }
  },
  {
    // RegExp
    tag: 27,
    // http://cbor.schmorp.de/generic-object
    encode(regex, encode3) {
      encode3(["RegExp", regex.source, regex.flags]);
    }
  },
  {
    // Tag
    getTag(tag) {
      return tag.tag;
    },
    encode(tag, encode3) {
      encode3(tag.value);
    }
  },
  {
    // ArrayBuffer
    encode(arrayBuffer, encode3, makeRoom) {
      writeBuffer(arrayBuffer, makeRoom);
    }
  },
  {
    // Uint8Array
    getTag(typedArray) {
      if (typedArray.constructor === Uint8Array) {
        if (this.tagUint8Array || hasNodeBuffer && this.tagUint8Array !== false)
          return 64;
      }
    },
    encode(typedArray, encode3, makeRoom) {
      writeBuffer(typedArray, makeRoom);
    }
  },
  typedArrayEncoder(68, 1),
  typedArrayEncoder(69, 2),
  typedArrayEncoder(70, 4),
  typedArrayEncoder(71, 8),
  typedArrayEncoder(72, 1),
  typedArrayEncoder(77, 2),
  typedArrayEncoder(78, 4),
  typedArrayEncoder(79, 8),
  typedArrayEncoder(85, 4),
  typedArrayEncoder(86, 8),
  {
    encode(sharedData, encode3) {
      let packedValues3 = sharedData.packedValues || [];
      let sharedStructures = sharedData.structures || [];
      if (packedValues3.values.length > 0) {
        target[position2++] = 216;
        target[position2++] = 51;
        writeArrayHeader(4);
        let valuesArray = packedValues3.values;
        encode3(valuesArray);
        writeArrayHeader(0);
        writeArrayHeader(0);
        packedObjectMap = Object.create(sharedPackedObjectMap || null);
        for (let i = 0, l = valuesArray.length; i < l; i++) {
          packedObjectMap[valuesArray[i]] = i;
        }
      }
      if (sharedStructures) {
        targetView.setUint32(position2, 3655335424);
        position2 += 3;
        let definitions = sharedStructures.slice(0);
        definitions.unshift(57344);
        definitions.push(new Tag(sharedData.version, 1399353956));
        encode3(definitions);
      } else
        encode3(new Tag(sharedData.version, 1399353956));
    }
  }
];
function typedArrayEncoder(tag, size) {
  if (!isLittleEndianMachine2 && size > 1)
    tag -= 4;
  return {
    tag,
    encode: function writeExtBuffer(typedArray, encode3) {
      let length = typedArray.byteLength;
      let offset = typedArray.byteOffset || 0;
      let buffer = typedArray.buffer || typedArray;
      encode3(hasNodeBuffer ? Buffer2.from(buffer, offset, length) : new Uint8Array(buffer, offset, length));
    }
  };
}
function writeBuffer(buffer, makeRoom) {
  let length = buffer.byteLength;
  if (length < 24) {
    target[position2++] = 64 + length;
  } else if (length < 256) {
    target[position2++] = 88;
    target[position2++] = length;
  } else if (length < 65536) {
    target[position2++] = 89;
    target[position2++] = length >> 8;
    target[position2++] = length & 255;
  } else {
    target[position2++] = 90;
    targetView.setUint32(position2, length);
    position2 += 4;
  }
  if (position2 + length >= target.length) {
    makeRoom(position2 + length);
  }
  target.set(buffer.buffer ? buffer : new Uint8Array(buffer), position2);
  position2 += length;
}
function insertIds(serialized, idsToInsert) {
  let nextId;
  let distanceToMove = idsToInsert.length * 2;
  let lastEnd = serialized.length - distanceToMove;
  idsToInsert.sort((a, b) => a.offset > b.offset ? 1 : -1);
  for (let id = 0; id < idsToInsert.length; id++) {
    let referee = idsToInsert[id];
    referee.id = id;
    for (let position5 of referee.references) {
      serialized[position5++] = id >> 8;
      serialized[position5] = id & 255;
    }
  }
  while (nextId = idsToInsert.pop()) {
    let offset = nextId.offset;
    serialized.copyWithin(offset + distanceToMove, offset, lastEnd);
    distanceToMove -= 2;
    let position5 = offset + distanceToMove;
    serialized[position5++] = 216;
    serialized[position5++] = 28;
    lastEnd = offset;
  }
  return serialized;
}
function writeBundles(start, encode3) {
  targetView.setUint32(bundledStrings2.position + start, position2 - bundledStrings2.position - start + 1);
  let writeStrings = bundledStrings2;
  bundledStrings2 = null;
  encode3(writeStrings[0]);
  encode3(writeStrings[1]);
}
var defaultEncoder = new Encoder({ useRecords: false });
var encode = defaultEncoder.encode;
var encodeAsIterable = defaultEncoder.encodeAsIterable;
var encodeAsAsyncIterable = defaultEncoder.encodeAsAsyncIterable;
var { NEVER, ALWAYS, DECIMAL_ROUND, DECIMAL_FIT } = FLOAT32_OPTIONS;
var REUSE_BUFFER_MODE = 512;
var RESET_BUFFER_MODE = 1024;
var THROW_ON_ITERABLE = 2048;

// wasi/callbuffer.ts
var CallBuffer = class {
  buffer;
  ctrl;
  len;
  data;
  maxData;
  constructor(buf) {
    this.buffer = buf;
    this.ctrl = new Int32Array(this.buffer, 0, 2);
    this.len = new Int32Array(this.buffer, 4, 1);
    this.data = new Uint8Array(this.buffer, 8);
    this.maxData = this.buffer.byteLength - 8 - 16;
  }
  respond(value) {
    let buf;
    if (value instanceof Uint8Array) {
      const limit = Math.min(value.length, this.maxData);
      buf = encode(value.slice(0, limit));
    } else {
      buf = encode(value);
    }
    this.len[0] = buf.length;
    this.data.set(buf, 0);
    Atomics.store(this.ctrl, 0, 1);
    Atomics.notify(this.ctrl, 0);
  }
  call(method, params) {
    this.ctrl[0] = 0;
    params["method"] = method;
    postMessage(params);
    Atomics.wait(this.ctrl, 0, 0);
    return decode(this.data.slice(0, this.len[0]));
  }
};

// node_modules/@bjorn3/browser_wasi_shim/dist/wasi_defs.js
var wasi_defs_exports = {};
__export(wasi_defs_exports, {
  ADVICE_DONTNEED: () => ADVICE_DONTNEED,
  ADVICE_NOREUSE: () => ADVICE_NOREUSE,
  ADVICE_NORMAL: () => ADVICE_NORMAL,
  ADVICE_RANDOM: () => ADVICE_RANDOM,
  ADVICE_SEQUENTIAL: () => ADVICE_SEQUENTIAL,
  ADVICE_WILLNEED: () => ADVICE_WILLNEED,
  CLOCKID_MONOTONIC: () => CLOCKID_MONOTONIC,
  CLOCKID_PROCESS_CPUTIME_ID: () => CLOCKID_PROCESS_CPUTIME_ID,
  CLOCKID_REALTIME: () => CLOCKID_REALTIME,
  CLOCKID_THREAD_CPUTIME_ID: () => CLOCKID_THREAD_CPUTIME_ID,
  Ciovec: () => Ciovec,
  Dirent: () => Dirent,
  ERRNO_2BIG: () => ERRNO_2BIG,
  ERRNO_ACCES: () => ERRNO_ACCES,
  ERRNO_ADDRINUSE: () => ERRNO_ADDRINUSE,
  ERRNO_ADDRNOTAVAIL: () => ERRNO_ADDRNOTAVAIL,
  ERRNO_AFNOSUPPORT: () => ERRNO_AFNOSUPPORT,
  ERRNO_AGAIN: () => ERRNO_AGAIN,
  ERRNO_ALREADY: () => ERRNO_ALREADY,
  ERRNO_BADF: () => ERRNO_BADF,
  ERRNO_BADMSG: () => ERRNO_BADMSG,
  ERRNO_BUSY: () => ERRNO_BUSY,
  ERRNO_CANCELED: () => ERRNO_CANCELED,
  ERRNO_CHILD: () => ERRNO_CHILD,
  ERRNO_CONNABORTED: () => ERRNO_CONNABORTED,
  ERRNO_CONNREFUSED: () => ERRNO_CONNREFUSED,
  ERRNO_CONNRESET: () => ERRNO_CONNRESET,
  ERRNO_DEADLK: () => ERRNO_DEADLK,
  ERRNO_DESTADDRREQ: () => ERRNO_DESTADDRREQ,
  ERRNO_DOM: () => ERRNO_DOM,
  ERRNO_DQUOT: () => ERRNO_DQUOT,
  ERRNO_EXIST: () => ERRNO_EXIST,
  ERRNO_FAULT: () => ERRNO_FAULT,
  ERRNO_FBIG: () => ERRNO_FBIG,
  ERRNO_HOSTUNREACH: () => ERRNO_HOSTUNREACH,
  ERRNO_IDRM: () => ERRNO_IDRM,
  ERRNO_ILSEQ: () => ERRNO_ILSEQ,
  ERRNO_INPROGRESS: () => ERRNO_INPROGRESS,
  ERRNO_INTR: () => ERRNO_INTR,
  ERRNO_INVAL: () => ERRNO_INVAL,
  ERRNO_IO: () => ERRNO_IO,
  ERRNO_ISCONN: () => ERRNO_ISCONN,
  ERRNO_ISDIR: () => ERRNO_ISDIR,
  ERRNO_LOOP: () => ERRNO_LOOP,
  ERRNO_MFILE: () => ERRNO_MFILE,
  ERRNO_MLINK: () => ERRNO_MLINK,
  ERRNO_MSGSIZE: () => ERRNO_MSGSIZE,
  ERRNO_MULTIHOP: () => ERRNO_MULTIHOP,
  ERRNO_NAMETOOLONG: () => ERRNO_NAMETOOLONG,
  ERRNO_NETDOWN: () => ERRNO_NETDOWN,
  ERRNO_NETRESET: () => ERRNO_NETRESET,
  ERRNO_NETUNREACH: () => ERRNO_NETUNREACH,
  ERRNO_NFILE: () => ERRNO_NFILE,
  ERRNO_NOBUFS: () => ERRNO_NOBUFS,
  ERRNO_NODEV: () => ERRNO_NODEV,
  ERRNO_NOENT: () => ERRNO_NOENT,
  ERRNO_NOEXEC: () => ERRNO_NOEXEC,
  ERRNO_NOLCK: () => ERRNO_NOLCK,
  ERRNO_NOLINK: () => ERRNO_NOLINK,
  ERRNO_NOMEM: () => ERRNO_NOMEM,
  ERRNO_NOMSG: () => ERRNO_NOMSG,
  ERRNO_NOPROTOOPT: () => ERRNO_NOPROTOOPT,
  ERRNO_NOSPC: () => ERRNO_NOSPC,
  ERRNO_NOSYS: () => ERRNO_NOSYS,
  ERRNO_NOTCAPABLE: () => ERRNO_NOTCAPABLE,
  ERRNO_NOTCONN: () => ERRNO_NOTCONN,
  ERRNO_NOTDIR: () => ERRNO_NOTDIR,
  ERRNO_NOTEMPTY: () => ERRNO_NOTEMPTY,
  ERRNO_NOTRECOVERABLE: () => ERRNO_NOTRECOVERABLE,
  ERRNO_NOTSOCK: () => ERRNO_NOTSOCK,
  ERRNO_NOTSUP: () => ERRNO_NOTSUP,
  ERRNO_NOTTY: () => ERRNO_NOTTY,
  ERRNO_NXIO: () => ERRNO_NXIO,
  ERRNO_OVERFLOW: () => ERRNO_OVERFLOW,
  ERRNO_OWNERDEAD: () => ERRNO_OWNERDEAD,
  ERRNO_PERM: () => ERRNO_PERM,
  ERRNO_PIPE: () => ERRNO_PIPE,
  ERRNO_PROTO: () => ERRNO_PROTO,
  ERRNO_PROTONOSUPPORT: () => ERRNO_PROTONOSUPPORT,
  ERRNO_PROTOTYPE: () => ERRNO_PROTOTYPE,
  ERRNO_RANGE: () => ERRNO_RANGE,
  ERRNO_ROFS: () => ERRNO_ROFS,
  ERRNO_SPIPE: () => ERRNO_SPIPE,
  ERRNO_SRCH: () => ERRNO_SRCH,
  ERRNO_STALE: () => ERRNO_STALE,
  ERRNO_SUCCESS: () => ERRNO_SUCCESS,
  ERRNO_TIMEDOUT: () => ERRNO_TIMEDOUT,
  ERRNO_TXTBSY: () => ERRNO_TXTBSY,
  ERRNO_XDEV: () => ERRNO_XDEV,
  EVENTRWFLAGS_FD_READWRITE_HANGUP: () => EVENTRWFLAGS_FD_READWRITE_HANGUP,
  EVENTTYPE_CLOCK: () => EVENTTYPE_CLOCK,
  EVENTTYPE_FD_READ: () => EVENTTYPE_FD_READ,
  EVENTTYPE_FD_WRITE: () => EVENTTYPE_FD_WRITE,
  Event: () => Event,
  FDFLAGS_APPEND: () => FDFLAGS_APPEND,
  FDFLAGS_DSYNC: () => FDFLAGS_DSYNC,
  FDFLAGS_NONBLOCK: () => FDFLAGS_NONBLOCK,
  FDFLAGS_RSYNC: () => FDFLAGS_RSYNC,
  FDFLAGS_SYNC: () => FDFLAGS_SYNC,
  FD_STDERR: () => FD_STDERR,
  FD_STDIN: () => FD_STDIN,
  FD_STDOUT: () => FD_STDOUT,
  FILETYPE_BLOCK_DEVICE: () => FILETYPE_BLOCK_DEVICE,
  FILETYPE_CHARACTER_DEVICE: () => FILETYPE_CHARACTER_DEVICE,
  FILETYPE_DIRECTORY: () => FILETYPE_DIRECTORY,
  FILETYPE_REGULAR_FILE: () => FILETYPE_REGULAR_FILE,
  FILETYPE_SOCKET_DGRAM: () => FILETYPE_SOCKET_DGRAM,
  FILETYPE_SOCKET_STREAM: () => FILETYPE_SOCKET_STREAM,
  FILETYPE_SYMBOLIC_LINK: () => FILETYPE_SYMBOLIC_LINK,
  FILETYPE_UNKNOWN: () => FILETYPE_UNKNOWN,
  FSTFLAGS_ATIM: () => FSTFLAGS_ATIM,
  FSTFLAGS_ATIM_NOW: () => FSTFLAGS_ATIM_NOW,
  FSTFLAGS_MTIM: () => FSTFLAGS_MTIM,
  FSTFLAGS_MTIM_NOW: () => FSTFLAGS_MTIM_NOW,
  Fdstat: () => Fdstat,
  Filestat: () => Filestat,
  Iovec: () => Iovec,
  OFLAGS_CREAT: () => OFLAGS_CREAT,
  OFLAGS_DIRECTORY: () => OFLAGS_DIRECTORY,
  OFLAGS_EXCL: () => OFLAGS_EXCL,
  OFLAGS_TRUNC: () => OFLAGS_TRUNC,
  PREOPENTYPE_DIR: () => PREOPENTYPE_DIR,
  Prestat: () => Prestat,
  PrestatDir: () => PrestatDir,
  RIFLAGS_RECV_PEEK: () => RIFLAGS_RECV_PEEK,
  RIFLAGS_RECV_WAITALL: () => RIFLAGS_RECV_WAITALL,
  RIGHTS_FD_ADVISE: () => RIGHTS_FD_ADVISE,
  RIGHTS_FD_ALLOCATE: () => RIGHTS_FD_ALLOCATE,
  RIGHTS_FD_DATASYNC: () => RIGHTS_FD_DATASYNC,
  RIGHTS_FD_FDSTAT_SET_FLAGS: () => RIGHTS_FD_FDSTAT_SET_FLAGS,
  RIGHTS_FD_FILESTAT_GET: () => RIGHTS_FD_FILESTAT_GET,
  RIGHTS_FD_FILESTAT_SET_SIZE: () => RIGHTS_FD_FILESTAT_SET_SIZE,
  RIGHTS_FD_FILESTAT_SET_TIMES: () => RIGHTS_FD_FILESTAT_SET_TIMES,
  RIGHTS_FD_READ: () => RIGHTS_FD_READ,
  RIGHTS_FD_READDIR: () => RIGHTS_FD_READDIR,
  RIGHTS_FD_SEEK: () => RIGHTS_FD_SEEK,
  RIGHTS_FD_SYNC: () => RIGHTS_FD_SYNC,
  RIGHTS_FD_TELL: () => RIGHTS_FD_TELL,
  RIGHTS_FD_WRITE: () => RIGHTS_FD_WRITE,
  RIGHTS_PATH_CREATE_DIRECTORY: () => RIGHTS_PATH_CREATE_DIRECTORY,
  RIGHTS_PATH_CREATE_FILE: () => RIGHTS_PATH_CREATE_FILE,
  RIGHTS_PATH_FILESTAT_GET: () => RIGHTS_PATH_FILESTAT_GET,
  RIGHTS_PATH_FILESTAT_SET_SIZE: () => RIGHTS_PATH_FILESTAT_SET_SIZE,
  RIGHTS_PATH_FILESTAT_SET_TIMES: () => RIGHTS_PATH_FILESTAT_SET_TIMES,
  RIGHTS_PATH_LINK_SOURCE: () => RIGHTS_PATH_LINK_SOURCE,
  RIGHTS_PATH_LINK_TARGET: () => RIGHTS_PATH_LINK_TARGET,
  RIGHTS_PATH_OPEN: () => RIGHTS_PATH_OPEN,
  RIGHTS_PATH_READLINK: () => RIGHTS_PATH_READLINK,
  RIGHTS_PATH_REMOVE_DIRECTORY: () => RIGHTS_PATH_REMOVE_DIRECTORY,
  RIGHTS_PATH_RENAME_SOURCE: () => RIGHTS_PATH_RENAME_SOURCE,
  RIGHTS_PATH_RENAME_TARGET: () => RIGHTS_PATH_RENAME_TARGET,
  RIGHTS_PATH_SYMLINK: () => RIGHTS_PATH_SYMLINK,
  RIGHTS_PATH_UNLINK_FILE: () => RIGHTS_PATH_UNLINK_FILE,
  RIGHTS_POLL_FD_READWRITE: () => RIGHTS_POLL_FD_READWRITE,
  RIGHTS_SOCK_SHUTDOWN: () => RIGHTS_SOCK_SHUTDOWN,
  ROFLAGS_RECV_DATA_TRUNCATED: () => ROFLAGS_RECV_DATA_TRUNCATED,
  SDFLAGS_RD: () => SDFLAGS_RD,
  SDFLAGS_WR: () => SDFLAGS_WR,
  SIGNAL_ABRT: () => SIGNAL_ABRT,
  SIGNAL_ALRM: () => SIGNAL_ALRM,
  SIGNAL_BUS: () => SIGNAL_BUS,
  SIGNAL_CHLD: () => SIGNAL_CHLD,
  SIGNAL_CONT: () => SIGNAL_CONT,
  SIGNAL_FPE: () => SIGNAL_FPE,
  SIGNAL_HUP: () => SIGNAL_HUP,
  SIGNAL_ILL: () => SIGNAL_ILL,
  SIGNAL_INT: () => SIGNAL_INT,
  SIGNAL_KILL: () => SIGNAL_KILL,
  SIGNAL_NONE: () => SIGNAL_NONE,
  SIGNAL_PIPE: () => SIGNAL_PIPE,
  SIGNAL_POLL: () => SIGNAL_POLL,
  SIGNAL_PROF: () => SIGNAL_PROF,
  SIGNAL_PWR: () => SIGNAL_PWR,
  SIGNAL_QUIT: () => SIGNAL_QUIT,
  SIGNAL_SEGV: () => SIGNAL_SEGV,
  SIGNAL_STOP: () => SIGNAL_STOP,
  SIGNAL_SYS: () => SIGNAL_SYS,
  SIGNAL_TERM: () => SIGNAL_TERM,
  SIGNAL_TRAP: () => SIGNAL_TRAP,
  SIGNAL_TSTP: () => SIGNAL_TSTP,
  SIGNAL_TTIN: () => SIGNAL_TTIN,
  SIGNAL_TTOU: () => SIGNAL_TTOU,
  SIGNAL_URG: () => SIGNAL_URG,
  SIGNAL_USR1: () => SIGNAL_USR1,
  SIGNAL_USR2: () => SIGNAL_USR2,
  SIGNAL_VTALRM: () => SIGNAL_VTALRM,
  SIGNAL_WINCH: () => SIGNAL_WINCH,
  SIGNAL_XCPU: () => SIGNAL_XCPU,
  SIGNAL_XFSZ: () => SIGNAL_XFSZ,
  SUBCLOCKFLAGS_SUBSCRIPTION_CLOCK_ABSTIME: () => SUBCLOCKFLAGS_SUBSCRIPTION_CLOCK_ABSTIME,
  Subscription: () => Subscription,
  WHENCE_CUR: () => WHENCE_CUR,
  WHENCE_END: () => WHENCE_END,
  WHENCE_SET: () => WHENCE_SET
});
var FD_STDIN = 0;
var FD_STDOUT = 1;
var FD_STDERR = 2;
var CLOCKID_REALTIME = 0;
var CLOCKID_MONOTONIC = 1;
var CLOCKID_PROCESS_CPUTIME_ID = 2;
var CLOCKID_THREAD_CPUTIME_ID = 3;
var ERRNO_SUCCESS = 0;
var ERRNO_2BIG = 1;
var ERRNO_ACCES = 2;
var ERRNO_ADDRINUSE = 3;
var ERRNO_ADDRNOTAVAIL = 4;
var ERRNO_AFNOSUPPORT = 5;
var ERRNO_AGAIN = 6;
var ERRNO_ALREADY = 7;
var ERRNO_BADF = 8;
var ERRNO_BADMSG = 9;
var ERRNO_BUSY = 10;
var ERRNO_CANCELED = 11;
var ERRNO_CHILD = 12;
var ERRNO_CONNABORTED = 13;
var ERRNO_CONNREFUSED = 14;
var ERRNO_CONNRESET = 15;
var ERRNO_DEADLK = 16;
var ERRNO_DESTADDRREQ = 17;
var ERRNO_DOM = 18;
var ERRNO_DQUOT = 19;
var ERRNO_EXIST = 20;
var ERRNO_FAULT = 21;
var ERRNO_FBIG = 22;
var ERRNO_HOSTUNREACH = 23;
var ERRNO_IDRM = 24;
var ERRNO_ILSEQ = 25;
var ERRNO_INPROGRESS = 26;
var ERRNO_INTR = 27;
var ERRNO_INVAL = 28;
var ERRNO_IO = 29;
var ERRNO_ISCONN = 30;
var ERRNO_ISDIR = 31;
var ERRNO_LOOP = 32;
var ERRNO_MFILE = 33;
var ERRNO_MLINK = 34;
var ERRNO_MSGSIZE = 35;
var ERRNO_MULTIHOP = 36;
var ERRNO_NAMETOOLONG = 37;
var ERRNO_NETDOWN = 38;
var ERRNO_NETRESET = 39;
var ERRNO_NETUNREACH = 40;
var ERRNO_NFILE = 41;
var ERRNO_NOBUFS = 42;
var ERRNO_NODEV = 43;
var ERRNO_NOENT = 44;
var ERRNO_NOEXEC = 45;
var ERRNO_NOLCK = 46;
var ERRNO_NOLINK = 47;
var ERRNO_NOMEM = 48;
var ERRNO_NOMSG = 49;
var ERRNO_NOPROTOOPT = 50;
var ERRNO_NOSPC = 51;
var ERRNO_NOSYS = 52;
var ERRNO_NOTCONN = 53;
var ERRNO_NOTDIR = 54;
var ERRNO_NOTEMPTY = 55;
var ERRNO_NOTRECOVERABLE = 56;
var ERRNO_NOTSOCK = 57;
var ERRNO_NOTSUP = 58;
var ERRNO_NOTTY = 59;
var ERRNO_NXIO = 60;
var ERRNO_OVERFLOW = 61;
var ERRNO_OWNERDEAD = 62;
var ERRNO_PERM = 63;
var ERRNO_PIPE = 64;
var ERRNO_PROTO = 65;
var ERRNO_PROTONOSUPPORT = 66;
var ERRNO_PROTOTYPE = 67;
var ERRNO_RANGE = 68;
var ERRNO_ROFS = 69;
var ERRNO_SPIPE = 70;
var ERRNO_SRCH = 71;
var ERRNO_STALE = 72;
var ERRNO_TIMEDOUT = 73;
var ERRNO_TXTBSY = 74;
var ERRNO_XDEV = 75;
var ERRNO_NOTCAPABLE = 76;
var RIGHTS_FD_DATASYNC = 1 << 0;
var RIGHTS_FD_READ = 1 << 1;
var RIGHTS_FD_SEEK = 1 << 2;
var RIGHTS_FD_FDSTAT_SET_FLAGS = 1 << 3;
var RIGHTS_FD_SYNC = 1 << 4;
var RIGHTS_FD_TELL = 1 << 5;
var RIGHTS_FD_WRITE = 1 << 6;
var RIGHTS_FD_ADVISE = 1 << 7;
var RIGHTS_FD_ALLOCATE = 1 << 8;
var RIGHTS_PATH_CREATE_DIRECTORY = 1 << 9;
var RIGHTS_PATH_CREATE_FILE = 1 << 10;
var RIGHTS_PATH_LINK_SOURCE = 1 << 11;
var RIGHTS_PATH_LINK_TARGET = 1 << 12;
var RIGHTS_PATH_OPEN = 1 << 13;
var RIGHTS_FD_READDIR = 1 << 14;
var RIGHTS_PATH_READLINK = 1 << 15;
var RIGHTS_PATH_RENAME_SOURCE = 1 << 16;
var RIGHTS_PATH_RENAME_TARGET = 1 << 17;
var RIGHTS_PATH_FILESTAT_GET = 1 << 18;
var RIGHTS_PATH_FILESTAT_SET_SIZE = 1 << 19;
var RIGHTS_PATH_FILESTAT_SET_TIMES = 1 << 20;
var RIGHTS_FD_FILESTAT_GET = 1 << 21;
var RIGHTS_FD_FILESTAT_SET_SIZE = 1 << 22;
var RIGHTS_FD_FILESTAT_SET_TIMES = 1 << 23;
var RIGHTS_PATH_SYMLINK = 1 << 24;
var RIGHTS_PATH_REMOVE_DIRECTORY = 1 << 25;
var RIGHTS_PATH_UNLINK_FILE = 1 << 26;
var RIGHTS_POLL_FD_READWRITE = 1 << 27;
var RIGHTS_SOCK_SHUTDOWN = 1 << 28;
var Iovec = class _Iovec {
  static read_bytes(view, ptr) {
    const iovec = new _Iovec();
    iovec.buf = view.getUint32(ptr, true);
    iovec.buf_len = view.getUint32(ptr + 4, true);
    return iovec;
  }
  static read_bytes_array(view, ptr, len) {
    const iovecs = [];
    for (let i = 0; i < len; i++) {
      iovecs.push(_Iovec.read_bytes(view, ptr + 8 * i));
    }
    return iovecs;
  }
};
var Ciovec = class _Ciovec {
  static read_bytes(view, ptr) {
    const iovec = new _Ciovec();
    iovec.buf = view.getUint32(ptr, true);
    iovec.buf_len = view.getUint32(ptr + 4, true);
    return iovec;
  }
  static read_bytes_array(view, ptr, len) {
    const iovecs = [];
    for (let i = 0; i < len; i++) {
      iovecs.push(_Ciovec.read_bytes(view, ptr + 8 * i));
    }
    return iovecs;
  }
};
var WHENCE_SET = 0;
var WHENCE_CUR = 1;
var WHENCE_END = 2;
var FILETYPE_UNKNOWN = 0;
var FILETYPE_BLOCK_DEVICE = 1;
var FILETYPE_CHARACTER_DEVICE = 2;
var FILETYPE_DIRECTORY = 3;
var FILETYPE_REGULAR_FILE = 4;
var FILETYPE_SOCKET_DGRAM = 5;
var FILETYPE_SOCKET_STREAM = 6;
var FILETYPE_SYMBOLIC_LINK = 7;
var Dirent = class {
  head_length() {
    return 24;
  }
  name_length() {
    return this.dir_name.byteLength;
  }
  write_head_bytes(view, ptr) {
    view.setBigUint64(ptr, this.d_next, true);
    view.setBigUint64(ptr + 8, this.d_ino, true);
    view.setUint32(ptr + 16, this.dir_name.length, true);
    view.setUint8(ptr + 20, this.d_type);
  }
  write_name_bytes(view8, ptr, buf_len) {
    view8.set(this.dir_name.slice(0, Math.min(this.dir_name.byteLength, buf_len)), ptr);
  }
  constructor(next_cookie, d_ino, name, type) {
    const encoded_name = new TextEncoder().encode(name);
    this.d_next = next_cookie;
    this.d_ino = d_ino;
    this.d_namlen = encoded_name.byteLength;
    this.d_type = type;
    this.dir_name = encoded_name;
  }
};
var ADVICE_NORMAL = 0;
var ADVICE_SEQUENTIAL = 1;
var ADVICE_RANDOM = 2;
var ADVICE_WILLNEED = 3;
var ADVICE_DONTNEED = 4;
var ADVICE_NOREUSE = 5;
var FDFLAGS_APPEND = 1 << 0;
var FDFLAGS_DSYNC = 1 << 1;
var FDFLAGS_NONBLOCK = 1 << 2;
var FDFLAGS_RSYNC = 1 << 3;
var FDFLAGS_SYNC = 1 << 4;
var Fdstat = class {
  write_bytes(view, ptr) {
    view.setUint8(ptr, this.fs_filetype);
    view.setUint16(ptr + 2, this.fs_flags, true);
    view.setBigUint64(ptr + 8, this.fs_rights_base, true);
    view.setBigUint64(ptr + 16, this.fs_rights_inherited, true);
  }
  constructor(filetype, flags) {
    this.fs_rights_base = 0n;
    this.fs_rights_inherited = 0n;
    this.fs_filetype = filetype;
    this.fs_flags = flags;
  }
};
var FSTFLAGS_ATIM = 1 << 0;
var FSTFLAGS_ATIM_NOW = 1 << 1;
var FSTFLAGS_MTIM = 1 << 2;
var FSTFLAGS_MTIM_NOW = 1 << 3;
var OFLAGS_CREAT = 1 << 0;
var OFLAGS_DIRECTORY = 1 << 1;
var OFLAGS_EXCL = 1 << 2;
var OFLAGS_TRUNC = 1 << 3;
var Filestat = class {
  write_bytes(view, ptr) {
    view.setBigUint64(ptr, this.dev, true);
    view.setBigUint64(ptr + 8, this.ino, true);
    view.setUint8(ptr + 16, this.filetype);
    view.setBigUint64(ptr + 24, this.nlink, true);
    view.setBigUint64(ptr + 32, this.size, true);
    view.setBigUint64(ptr + 38, this.atim, true);
    view.setBigUint64(ptr + 46, this.mtim, true);
    view.setBigUint64(ptr + 52, this.ctim, true);
  }
  constructor(ino, filetype, size) {
    this.dev = 0n;
    this.nlink = 0n;
    this.atim = 0n;
    this.mtim = 0n;
    this.ctim = 0n;
    this.ino = ino;
    this.filetype = filetype;
    this.size = size;
  }
};
var EVENTTYPE_CLOCK = 0;
var EVENTTYPE_FD_READ = 1;
var EVENTTYPE_FD_WRITE = 2;
var EVENTRWFLAGS_FD_READWRITE_HANGUP = 1 << 0;
var SUBCLOCKFLAGS_SUBSCRIPTION_CLOCK_ABSTIME = 1 << 0;
var Subscription = class _Subscription {
  static read_bytes(view, ptr) {
    return new _Subscription(view.getBigUint64(ptr, true), view.getUint8(ptr + 8), view.getUint32(ptr + 16, true), view.getBigUint64(ptr + 24, true), view.getUint16(ptr + 36, true));
  }
  constructor(userdata, eventtype, clockid, timeout, flags) {
    this.userdata = userdata;
    this.eventtype = eventtype;
    this.clockid = clockid;
    this.timeout = timeout;
    this.flags = flags;
  }
};
var Event = class {
  write_bytes(view, ptr) {
    view.setBigUint64(ptr, this.userdata, true);
    view.setUint16(ptr + 8, this.error, true);
    view.setUint8(ptr + 10, this.eventtype);
  }
  constructor(userdata, error, eventtype) {
    this.userdata = userdata;
    this.error = error;
    this.eventtype = eventtype;
  }
};
var SIGNAL_NONE = 0;
var SIGNAL_HUP = 1;
var SIGNAL_INT = 2;
var SIGNAL_QUIT = 3;
var SIGNAL_ILL = 4;
var SIGNAL_TRAP = 5;
var SIGNAL_ABRT = 6;
var SIGNAL_BUS = 7;
var SIGNAL_FPE = 8;
var SIGNAL_KILL = 9;
var SIGNAL_USR1 = 10;
var SIGNAL_SEGV = 11;
var SIGNAL_USR2 = 12;
var SIGNAL_PIPE = 13;
var SIGNAL_ALRM = 14;
var SIGNAL_TERM = 15;
var SIGNAL_CHLD = 16;
var SIGNAL_CONT = 17;
var SIGNAL_STOP = 18;
var SIGNAL_TSTP = 19;
var SIGNAL_TTIN = 20;
var SIGNAL_TTOU = 21;
var SIGNAL_URG = 22;
var SIGNAL_XCPU = 23;
var SIGNAL_XFSZ = 24;
var SIGNAL_VTALRM = 25;
var SIGNAL_PROF = 26;
var SIGNAL_WINCH = 27;
var SIGNAL_POLL = 28;
var SIGNAL_PWR = 29;
var SIGNAL_SYS = 30;
var RIFLAGS_RECV_PEEK = 1 << 0;
var RIFLAGS_RECV_WAITALL = 1 << 1;
var ROFLAGS_RECV_DATA_TRUNCATED = 1 << 0;
var SDFLAGS_RD = 1 << 0;
var SDFLAGS_WR = 1 << 1;
var PREOPENTYPE_DIR = 0;
var PrestatDir = class {
  write_bytes(view, ptr) {
    view.setUint32(ptr, this.pr_name.byteLength, true);
  }
  constructor(name) {
    this.pr_name = new TextEncoder().encode(name);
  }
};
var Prestat = class _Prestat {
  static dir(name) {
    const prestat = new _Prestat();
    prestat.tag = PREOPENTYPE_DIR;
    prestat.inner = new PrestatDir(name);
    return prestat;
  }
  write_bytes(view, ptr) {
    view.setUint32(ptr, this.tag, true);
    this.inner.write_bytes(view, ptr + 4);
  }
};

// node_modules/@bjorn3/browser_wasi_shim/dist/debug.js
var Debug = class Debug2 {
  enable(enabled) {
    this.log = createLogger(enabled === void 0 ? true : enabled, this.prefix);
  }
  get enabled() {
    return this.isEnabled;
  }
  constructor(isEnabled) {
    this.isEnabled = isEnabled;
    this.prefix = "wasi:";
    this.enable(isEnabled);
  }
};
function createLogger(enabled, prefix) {
  if (enabled) {
    const a = console.log.bind(console, "%c%s", "color: #265BA0", prefix);
    return a;
  } else {
    return () => {
    };
  }
}
var debug = new Debug(false);

// node_modules/@bjorn3/browser_wasi_shim/dist/wasi.js
var WASIProcExit = class extends Error {
  constructor(code) {
    super("exit with exit code " + code);
    this.code = code;
  }
};
var WASI = class WASI2 {
  start(instance) {
    this.inst = instance;
    try {
      instance.exports._start();
      return 0;
    } catch (e) {
      if (e instanceof WASIProcExit) {
        return e.code;
      } else {
        throw e;
      }
    }
  }
  initialize(instance) {
    this.inst = instance;
    if (instance.exports._initialize) {
      instance.exports._initialize();
    }
  }
  constructor(args, env, fds, options = {}) {
    this.args = [];
    this.env = [];
    this.fds = [];
    debug.enable(options.debug);
    this.args = args;
    this.env = env;
    this.fds = fds;
    const self = this;
    this.wasiImport = { args_sizes_get(argc, argv_buf_size) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      buffer.setUint32(argc, self.args.length, true);
      let buf_size = 0;
      for (const arg of self.args) {
        buf_size += arg.length + 1;
      }
      buffer.setUint32(argv_buf_size, buf_size, true);
      debug.log(buffer.getUint32(argc, true), buffer.getUint32(argv_buf_size, true));
      return 0;
    }, args_get(argv, argv_buf) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      const orig_argv_buf = argv_buf;
      for (let i = 0; i < self.args.length; i++) {
        buffer.setUint32(argv, argv_buf, true);
        argv += 4;
        const arg = new TextEncoder().encode(self.args[i]);
        buffer8.set(arg, argv_buf);
        buffer.setUint8(argv_buf + arg.length, 0);
        argv_buf += arg.length + 1;
      }
      if (debug.enabled) {
        debug.log(new TextDecoder("utf-8").decode(buffer8.slice(orig_argv_buf, argv_buf)));
      }
      return 0;
    }, environ_sizes_get(environ_count, environ_size) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      buffer.setUint32(environ_count, self.env.length, true);
      let buf_size = 0;
      for (const environ of self.env) {
        buf_size += new TextEncoder().encode(environ).length + 1;
      }
      buffer.setUint32(environ_size, buf_size, true);
      debug.log(buffer.getUint32(environ_count, true), buffer.getUint32(environ_size, true));
      return 0;
    }, environ_get(environ, environ_buf) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      const orig_environ_buf = environ_buf;
      for (let i = 0; i < self.env.length; i++) {
        buffer.setUint32(environ, environ_buf, true);
        environ += 4;
        const e = new TextEncoder().encode(self.env[i]);
        buffer8.set(e, environ_buf);
        buffer.setUint8(environ_buf + e.length, 0);
        environ_buf += e.length + 1;
      }
      if (debug.enabled) {
        debug.log(new TextDecoder("utf-8").decode(buffer8.slice(orig_environ_buf, environ_buf)));
      }
      return 0;
    }, clock_res_get(id, res_ptr) {
      let resolutionValue;
      switch (id) {
        case CLOCKID_MONOTONIC: {
          resolutionValue = 5000n;
          break;
        }
        case CLOCKID_REALTIME: {
          resolutionValue = 1000000n;
          break;
        }
        default:
          return ERRNO_NOSYS;
      }
      const view = new DataView(self.inst.exports.memory.buffer);
      view.setBigUint64(res_ptr, resolutionValue, true);
      return ERRNO_SUCCESS;
    }, clock_time_get(id, precision, time) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      if (id === CLOCKID_REALTIME) {
        buffer.setBigUint64(time, BigInt((/* @__PURE__ */ new Date()).getTime()) * 1000000n, true);
      } else if (id == CLOCKID_MONOTONIC) {
        let monotonic_time;
        try {
          monotonic_time = BigInt(Math.round(performance.now() * 1e6));
        } catch (e) {
          monotonic_time = 0n;
        }
        buffer.setBigUint64(time, monotonic_time, true);
      } else {
        buffer.setBigUint64(time, 0n, true);
      }
      return 0;
    }, fd_advise(fd, offset, len, advice) {
      if (self.fds[fd] != void 0) {
        return ERRNO_SUCCESS;
      } else {
        return ERRNO_BADF;
      }
    }, fd_allocate(fd, offset, len) {
      if (self.fds[fd] != void 0) {
        return self.fds[fd].fd_allocate(offset, len);
      } else {
        return ERRNO_BADF;
      }
    }, fd_close(fd) {
      if (self.fds[fd] != void 0) {
        const ret = self.fds[fd].fd_close();
        self.fds[fd] = void 0;
        return ret;
      } else {
        return ERRNO_BADF;
      }
    }, fd_datasync(fd) {
      if (self.fds[fd] != void 0) {
        return self.fds[fd].fd_sync();
      } else {
        return ERRNO_BADF;
      }
    }, fd_fdstat_get(fd, fdstat_ptr) {
      if (self.fds[fd] != void 0) {
        const { ret, fdstat } = self.fds[fd].fd_fdstat_get();
        if (fdstat != null) {
          fdstat.write_bytes(new DataView(self.inst.exports.memory.buffer), fdstat_ptr);
        }
        return ret;
      } else {
        return ERRNO_BADF;
      }
    }, fd_fdstat_set_flags(fd, flags) {
      if (self.fds[fd] != void 0) {
        return self.fds[fd].fd_fdstat_set_flags(flags);
      } else {
        return ERRNO_BADF;
      }
    }, fd_fdstat_set_rights(fd, fs_rights_base, fs_rights_inheriting) {
      if (self.fds[fd] != void 0) {
        return self.fds[fd].fd_fdstat_set_rights(fs_rights_base, fs_rights_inheriting);
      } else {
        return ERRNO_BADF;
      }
    }, fd_filestat_get(fd, filestat_ptr) {
      if (self.fds[fd] != void 0) {
        const { ret, filestat } = self.fds[fd].fd_filestat_get();
        if (filestat != null) {
          filestat.write_bytes(new DataView(self.inst.exports.memory.buffer), filestat_ptr);
        }
        return ret;
      } else {
        return ERRNO_BADF;
      }
    }, fd_filestat_set_size(fd, size) {
      if (self.fds[fd] != void 0) {
        return self.fds[fd].fd_filestat_set_size(size);
      } else {
        return ERRNO_BADF;
      }
    }, fd_filestat_set_times(fd, atim, mtim, fst_flags) {
      if (self.fds[fd] != void 0) {
        return self.fds[fd].fd_filestat_set_times(atim, mtim, fst_flags);
      } else {
        return ERRNO_BADF;
      }
    }, fd_pread(fd, iovs_ptr, iovs_len, offset, nread_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const iovecs = Iovec.read_bytes_array(buffer, iovs_ptr, iovs_len);
        let nread = 0;
        for (const iovec of iovecs) {
          const { ret, data } = self.fds[fd].fd_pread(iovec.buf_len, offset);
          if (ret != ERRNO_SUCCESS) {
            buffer.setUint32(nread_ptr, nread, true);
            return ret;
          }
          buffer8.set(data, iovec.buf);
          nread += data.length;
          offset += BigInt(data.length);
          if (data.length != iovec.buf_len) {
            break;
          }
        }
        buffer.setUint32(nread_ptr, nread, true);
        return ERRNO_SUCCESS;
      } else {
        return ERRNO_BADF;
      }
    }, fd_prestat_get(fd, buf_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const { ret, prestat } = self.fds[fd].fd_prestat_get();
        if (prestat != null) {
          prestat.write_bytes(buffer, buf_ptr);
        }
        return ret;
      } else {
        return ERRNO_BADF;
      }
    }, fd_prestat_dir_name(fd, path_ptr, path_len) {
      if (self.fds[fd] != void 0) {
        const { ret, prestat } = self.fds[fd].fd_prestat_get();
        if (prestat == null) {
          return ret;
        }
        const prestat_dir_name = prestat.inner.pr_name;
        const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
        buffer8.set(prestat_dir_name.slice(0, path_len), path_ptr);
        return prestat_dir_name.byteLength > path_len ? ERRNO_NAMETOOLONG : ERRNO_SUCCESS;
      } else {
        return ERRNO_BADF;
      }
    }, fd_pwrite(fd, iovs_ptr, iovs_len, offset, nwritten_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const iovecs = Ciovec.read_bytes_array(buffer, iovs_ptr, iovs_len);
        let nwritten = 0;
        for (const iovec of iovecs) {
          const data = buffer8.slice(iovec.buf, iovec.buf + iovec.buf_len);
          const { ret, nwritten: nwritten_part } = self.fds[fd].fd_pwrite(data, offset);
          if (ret != ERRNO_SUCCESS) {
            buffer.setUint32(nwritten_ptr, nwritten, true);
            return ret;
          }
          nwritten += nwritten_part;
          offset += BigInt(nwritten_part);
          if (nwritten_part != data.byteLength) {
            break;
          }
        }
        buffer.setUint32(nwritten_ptr, nwritten, true);
        return ERRNO_SUCCESS;
      } else {
        return ERRNO_BADF;
      }
    }, fd_read(fd, iovs_ptr, iovs_len, nread_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const iovecs = Iovec.read_bytes_array(buffer, iovs_ptr, iovs_len);
        let nread = 0;
        for (const iovec of iovecs) {
          const { ret, data } = self.fds[fd].fd_read(iovec.buf_len);
          if (ret != ERRNO_SUCCESS) {
            buffer.setUint32(nread_ptr, nread, true);
            return ret;
          }
          buffer8.set(data, iovec.buf);
          nread += data.length;
          if (data.length != iovec.buf_len) {
            break;
          }
        }
        buffer.setUint32(nread_ptr, nread, true);
        return ERRNO_SUCCESS;
      } else {
        return ERRNO_BADF;
      }
    }, fd_readdir(fd, buf, buf_len, cookie, bufused_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        let bufused = 0;
        while (true) {
          const { ret, dirent } = self.fds[fd].fd_readdir_single(cookie);
          if (ret != 0) {
            buffer.setUint32(bufused_ptr, bufused, true);
            return ret;
          }
          if (dirent == null) {
            break;
          }
          if (buf_len - bufused < dirent.head_length()) {
            bufused = buf_len;
            break;
          }
          const head_bytes = new ArrayBuffer(dirent.head_length());
          dirent.write_head_bytes(new DataView(head_bytes), 0);
          buffer8.set(new Uint8Array(head_bytes).slice(0, Math.min(head_bytes.byteLength, buf_len - bufused)), buf);
          buf += dirent.head_length();
          bufused += dirent.head_length();
          if (buf_len - bufused < dirent.name_length()) {
            bufused = buf_len;
            break;
          }
          dirent.write_name_bytes(buffer8, buf, buf_len - bufused);
          buf += dirent.name_length();
          bufused += dirent.name_length();
          cookie = dirent.d_next;
        }
        buffer.setUint32(bufused_ptr, bufused, true);
        return 0;
      } else {
        return ERRNO_BADF;
      }
    }, fd_renumber(fd, to) {
      if (self.fds[fd] != void 0 && self.fds[to] != void 0) {
        const ret = self.fds[to].fd_close();
        if (ret != 0) {
          return ret;
        }
        self.fds[to] = self.fds[fd];
        self.fds[fd] = void 0;
        return 0;
      } else {
        return ERRNO_BADF;
      }
    }, fd_seek(fd, offset, whence, offset_out_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const { ret, offset: offset_out } = self.fds[fd].fd_seek(offset, whence);
        buffer.setBigInt64(offset_out_ptr, offset_out, true);
        return ret;
      } else {
        return ERRNO_BADF;
      }
    }, fd_sync(fd) {
      if (self.fds[fd] != void 0) {
        return self.fds[fd].fd_sync();
      } else {
        return ERRNO_BADF;
      }
    }, fd_tell(fd, offset_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const { ret, offset } = self.fds[fd].fd_tell();
        buffer.setBigUint64(offset_ptr, offset, true);
        return ret;
      } else {
        return ERRNO_BADF;
      }
    }, fd_write(fd, iovs_ptr, iovs_len, nwritten_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const iovecs = Ciovec.read_bytes_array(buffer, iovs_ptr, iovs_len);
        let nwritten = 0;
        for (const iovec of iovecs) {
          const data = buffer8.slice(iovec.buf, iovec.buf + iovec.buf_len);
          const { ret, nwritten: nwritten_part } = self.fds[fd].fd_write(data);
          if (ret != ERRNO_SUCCESS) {
            buffer.setUint32(nwritten_ptr, nwritten, true);
            return ret;
          }
          nwritten += nwritten_part;
          if (nwritten_part != data.byteLength) {
            break;
          }
        }
        buffer.setUint32(nwritten_ptr, nwritten, true);
        return ERRNO_SUCCESS;
      } else {
        return ERRNO_BADF;
      }
    }, path_create_directory(fd, path_ptr, path_len) {
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const path = new TextDecoder("utf-8").decode(buffer8.slice(path_ptr, path_ptr + path_len));
        return self.fds[fd].path_create_directory(path);
      } else {
        return ERRNO_BADF;
      }
    }, path_filestat_get(fd, flags, path_ptr, path_len, filestat_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const path = new TextDecoder("utf-8").decode(buffer8.slice(path_ptr, path_ptr + path_len));
        const { ret, filestat } = self.fds[fd].path_filestat_get(flags, path);
        if (filestat != null) {
          filestat.write_bytes(buffer, filestat_ptr);
        }
        return ret;
      } else {
        return ERRNO_BADF;
      }
    }, path_filestat_set_times(fd, flags, path_ptr, path_len, atim, mtim, fst_flags) {
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const path = new TextDecoder("utf-8").decode(buffer8.slice(path_ptr, path_ptr + path_len));
        return self.fds[fd].path_filestat_set_times(flags, path, atim, mtim, fst_flags);
      } else {
        return ERRNO_BADF;
      }
    }, path_link(old_fd, old_flags, old_path_ptr, old_path_len, new_fd, new_path_ptr, new_path_len) {
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[old_fd] != void 0 && self.fds[new_fd] != void 0) {
        const old_path = new TextDecoder("utf-8").decode(buffer8.slice(old_path_ptr, old_path_ptr + old_path_len));
        const new_path = new TextDecoder("utf-8").decode(buffer8.slice(new_path_ptr, new_path_ptr + new_path_len));
        const { ret, inode_obj } = self.fds[old_fd].path_lookup(old_path, old_flags);
        if (inode_obj == null) {
          return ret;
        }
        return self.fds[new_fd].path_link(new_path, inode_obj, false);
      } else {
        return ERRNO_BADF;
      }
    }, path_open(fd, dirflags, path_ptr, path_len, oflags, fs_rights_base, fs_rights_inheriting, fd_flags, opened_fd_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const path = new TextDecoder("utf-8").decode(buffer8.slice(path_ptr, path_ptr + path_len));
        debug.log(path);
        const { ret, fd_obj } = self.fds[fd].path_open(dirflags, path, oflags, fs_rights_base, fs_rights_inheriting, fd_flags);
        if (ret != 0) {
          return ret;
        }
        self.fds.push(fd_obj);
        const opened_fd = self.fds.length - 1;
        buffer.setUint32(opened_fd_ptr, opened_fd, true);
        return 0;
      } else {
        return ERRNO_BADF;
      }
    }, path_readlink(fd, path_ptr, path_len, buf_ptr, buf_len, nread_ptr) {
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const path = new TextDecoder("utf-8").decode(buffer8.slice(path_ptr, path_ptr + path_len));
        debug.log(path);
        const { ret, data } = self.fds[fd].path_readlink(path);
        if (data != null) {
          const data_buf = new TextEncoder().encode(data);
          if (data_buf.length > buf_len) {
            buffer.setUint32(nread_ptr, 0, true);
            return ERRNO_BADF;
          }
          buffer8.set(data_buf, buf_ptr);
          buffer.setUint32(nread_ptr, data_buf.length, true);
        }
        return ret;
      } else {
        return ERRNO_BADF;
      }
    }, path_remove_directory(fd, path_ptr, path_len) {
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const path = new TextDecoder("utf-8").decode(buffer8.slice(path_ptr, path_ptr + path_len));
        return self.fds[fd].path_remove_directory(path);
      } else {
        return ERRNO_BADF;
      }
    }, path_rename(fd, old_path_ptr, old_path_len, new_fd, new_path_ptr, new_path_len) {
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0 && self.fds[new_fd] != void 0) {
        const old_path = new TextDecoder("utf-8").decode(buffer8.slice(old_path_ptr, old_path_ptr + old_path_len));
        const new_path = new TextDecoder("utf-8").decode(buffer8.slice(new_path_ptr, new_path_ptr + new_path_len));
        let { ret, inode_obj } = self.fds[fd].path_unlink(old_path);
        if (inode_obj == null) {
          return ret;
        }
        ret = self.fds[new_fd].path_link(new_path, inode_obj, true);
        if (ret != ERRNO_SUCCESS) {
          if (self.fds[fd].path_link(old_path, inode_obj, true) != ERRNO_SUCCESS) {
            throw "path_link should always return success when relinking an inode back to the original place";
          }
        }
        return ret;
      } else {
        return ERRNO_BADF;
      }
    }, path_symlink(old_path_ptr, old_path_len, fd, new_path_ptr, new_path_len) {
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const old_path = new TextDecoder("utf-8").decode(buffer8.slice(old_path_ptr, old_path_ptr + old_path_len));
        const new_path = new TextDecoder("utf-8").decode(buffer8.slice(new_path_ptr, new_path_ptr + new_path_len));
        return ERRNO_NOTSUP;
      } else {
        return ERRNO_BADF;
      }
    }, path_unlink_file(fd, path_ptr, path_len) {
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer);
      if (self.fds[fd] != void 0) {
        const path = new TextDecoder("utf-8").decode(buffer8.slice(path_ptr, path_ptr + path_len));
        return self.fds[fd].path_unlink_file(path);
      } else {
        return ERRNO_BADF;
      }
    }, poll_oneoff(in_ptr, out_ptr, nsubscriptions) {
      if (nsubscriptions === 0) {
        return ERRNO_INVAL;
      }
      if (nsubscriptions > 1) {
        debug.log("poll_oneoff: only a single subscription is supported");
        return ERRNO_NOTSUP;
      }
      const buffer = new DataView(self.inst.exports.memory.buffer);
      const s = Subscription.read_bytes(buffer, in_ptr);
      const eventtype = s.eventtype;
      const clockid = s.clockid;
      const timeout = s.timeout;
      if (eventtype !== EVENTTYPE_CLOCK) {
        debug.log("poll_oneoff: only clock subscriptions are supported");
        return ERRNO_NOTSUP;
      }
      let getNow = void 0;
      if (clockid === CLOCKID_MONOTONIC) {
        getNow = () => BigInt(Math.round(performance.now() * 1e6));
      } else if (clockid === CLOCKID_REALTIME) {
        getNow = () => BigInt((/* @__PURE__ */ new Date()).getTime()) * 1000000n;
      } else {
        return ERRNO_INVAL;
      }
      const endTime = (s.flags & SUBCLOCKFLAGS_SUBSCRIPTION_CLOCK_ABSTIME) !== 0 ? timeout : getNow() + timeout;
      while (endTime > getNow()) {
      }
      const event = new Event(s.userdata, ERRNO_SUCCESS, eventtype);
      event.write_bytes(buffer, out_ptr);
      return ERRNO_SUCCESS;
    }, proc_exit(exit_code) {
      throw new WASIProcExit(exit_code);
    }, proc_raise(sig) {
      throw "raised signal " + sig;
    }, sched_yield() {
    }, random_get(buf, buf_len) {
      const buffer8 = new Uint8Array(self.inst.exports.memory.buffer).subarray(buf, buf + buf_len);
      if ("crypto" in globalThis && (typeof SharedArrayBuffer === "undefined" || !(self.inst.exports.memory.buffer instanceof SharedArrayBuffer))) {
        for (let i = 0; i < buf_len; i += 65536) {
          crypto.getRandomValues(buffer8.subarray(i, i + 65536));
        }
      } else {
        for (let i = 0; i < buf_len; i++) {
          buffer8[i] = Math.random() * 256 | 0;
        }
      }
    }, sock_recv(fd, ri_data, ri_flags) {
      throw "sockets not supported";
    }, sock_send(fd, si_data, si_flags) {
      throw "sockets not supported";
    }, sock_shutdown(fd, how) {
      throw "sockets not supported";
    }, sock_accept(fd, flags) {
      throw "sockets not supported";
    } };
  }
};

// node_modules/@bjorn3/browser_wasi_shim/dist/fd.js
var Fd = class {
  fd_allocate(offset, len) {
    return ERRNO_NOTSUP;
  }
  fd_close() {
    return 0;
  }
  fd_fdstat_get() {
    return { ret: ERRNO_NOTSUP, fdstat: null };
  }
  fd_fdstat_set_flags(flags) {
    return ERRNO_NOTSUP;
  }
  fd_fdstat_set_rights(fs_rights_base, fs_rights_inheriting) {
    return ERRNO_NOTSUP;
  }
  fd_filestat_get() {
    return { ret: ERRNO_NOTSUP, filestat: null };
  }
  fd_filestat_set_size(size) {
    return ERRNO_NOTSUP;
  }
  fd_filestat_set_times(atim, mtim, fst_flags) {
    return ERRNO_NOTSUP;
  }
  fd_pread(size, offset) {
    return { ret: ERRNO_NOTSUP, data: new Uint8Array() };
  }
  fd_prestat_get() {
    return { ret: ERRNO_NOTSUP, prestat: null };
  }
  fd_pwrite(data, offset) {
    return { ret: ERRNO_NOTSUP, nwritten: 0 };
  }
  fd_read(size) {
    return { ret: ERRNO_NOTSUP, data: new Uint8Array() };
  }
  fd_readdir_single(cookie) {
    return { ret: ERRNO_NOTSUP, dirent: null };
  }
  fd_seek(offset, whence) {
    return { ret: ERRNO_NOTSUP, offset: 0n };
  }
  fd_sync() {
    return 0;
  }
  fd_tell() {
    return { ret: ERRNO_NOTSUP, offset: 0n };
  }
  fd_write(data) {
    return { ret: ERRNO_NOTSUP, nwritten: 0 };
  }
  path_create_directory(path) {
    return ERRNO_NOTSUP;
  }
  path_filestat_get(flags, path) {
    return { ret: ERRNO_NOTSUP, filestat: null };
  }
  path_filestat_set_times(flags, path, atim, mtim, fst_flags) {
    return ERRNO_NOTSUP;
  }
  path_link(path, inode, allow_dir) {
    return ERRNO_NOTSUP;
  }
  path_unlink(path) {
    return { ret: ERRNO_NOTSUP, inode_obj: null };
  }
  path_lookup(path, dirflags) {
    return { ret: ERRNO_NOTSUP, inode_obj: null };
  }
  path_open(dirflags, path, oflags, fs_rights_base, fs_rights_inheriting, fd_flags) {
    return { ret: ERRNO_NOTDIR, fd_obj: null };
  }
  path_readlink(path) {
    return { ret: ERRNO_NOTSUP, data: null };
  }
  path_remove_directory(path) {
    return ERRNO_NOTSUP;
  }
  path_rename(old_path, new_fd, new_path) {
    return ERRNO_NOTSUP;
  }
  path_unlink_file(path) {
    return ERRNO_NOTSUP;
  }
};
var Inode = class _Inode {
  static issue_ino() {
    return _Inode.next_ino++;
  }
  static root_ino() {
    return 0n;
  }
  constructor() {
    this.ino = _Inode.issue_ino();
  }
};
Inode.next_ino = 1n;

// node_modules/@bjorn3/browser_wasi_shim/dist/fs_mem.js
var OpenFile = class extends Fd {
  fd_allocate(offset, len) {
    if (this.file.size > offset + len) {
    } else {
      const new_data = new Uint8Array(Number(offset + len));
      new_data.set(this.file.data, 0);
      this.file.data = new_data;
    }
    return ERRNO_SUCCESS;
  }
  fd_fdstat_get() {
    return { ret: 0, fdstat: new Fdstat(FILETYPE_REGULAR_FILE, 0) };
  }
  fd_filestat_set_size(size) {
    if (this.file.size > size) {
      this.file.data = new Uint8Array(this.file.data.buffer.slice(0, Number(size)));
    } else {
      const new_data = new Uint8Array(Number(size));
      new_data.set(this.file.data, 0);
      this.file.data = new_data;
    }
    return ERRNO_SUCCESS;
  }
  fd_read(size) {
    const slice = this.file.data.slice(Number(this.file_pos), Number(this.file_pos + BigInt(size)));
    this.file_pos += BigInt(slice.length);
    return { ret: 0, data: slice };
  }
  fd_pread(size, offset) {
    const slice = this.file.data.slice(Number(offset), Number(offset + BigInt(size)));
    return { ret: 0, data: slice };
  }
  fd_seek(offset, whence) {
    let calculated_offset;
    switch (whence) {
      case WHENCE_SET:
        calculated_offset = offset;
        break;
      case WHENCE_CUR:
        calculated_offset = this.file_pos + offset;
        break;
      case WHENCE_END:
        calculated_offset = BigInt(this.file.data.byteLength) + offset;
        break;
      default:
        return { ret: ERRNO_INVAL, offset: 0n };
    }
    if (calculated_offset < 0) {
      return { ret: ERRNO_INVAL, offset: 0n };
    }
    this.file_pos = calculated_offset;
    return { ret: 0, offset: this.file_pos };
  }
  fd_tell() {
    return { ret: 0, offset: this.file_pos };
  }
  fd_write(data) {
    if (this.file.readonly) return { ret: ERRNO_BADF, nwritten: 0 };
    if (this.file_pos + BigInt(data.byteLength) > this.file.size) {
      const old = this.file.data;
      this.file.data = new Uint8Array(Number(this.file_pos + BigInt(data.byteLength)));
      this.file.data.set(old);
    }
    this.file.data.set(data, Number(this.file_pos));
    this.file_pos += BigInt(data.byteLength);
    return { ret: 0, nwritten: data.byteLength };
  }
  fd_pwrite(data, offset) {
    if (this.file.readonly) return { ret: ERRNO_BADF, nwritten: 0 };
    if (offset + BigInt(data.byteLength) > this.file.size) {
      const old = this.file.data;
      this.file.data = new Uint8Array(Number(offset + BigInt(data.byteLength)));
      this.file.data.set(old);
    }
    this.file.data.set(data, Number(offset));
    return { ret: 0, nwritten: data.byteLength };
  }
  fd_filestat_get() {
    return { ret: 0, filestat: this.file.stat() };
  }
  constructor(file) {
    super();
    this.file_pos = 0n;
    this.file = file;
  }
};
var File = class extends Inode {
  path_open(oflags, fs_rights_base, fd_flags) {
    if (this.readonly && (fs_rights_base & BigInt(RIGHTS_FD_WRITE)) == BigInt(RIGHTS_FD_WRITE)) {
      return { ret: ERRNO_PERM, fd_obj: null };
    }
    if ((oflags & OFLAGS_TRUNC) == OFLAGS_TRUNC) {
      if (this.readonly) return { ret: ERRNO_PERM, fd_obj: null };
      this.data = new Uint8Array([]);
    }
    const file = new OpenFile(this);
    if (fd_flags & FDFLAGS_APPEND) file.fd_seek(0n, WHENCE_END);
    return { ret: ERRNO_SUCCESS, fd_obj: file };
  }
  get size() {
    return BigInt(this.data.byteLength);
  }
  stat() {
    return new Filestat(this.ino, FILETYPE_REGULAR_FILE, this.size);
  }
  constructor(data, options) {
    super();
    this.data = new Uint8Array(data);
    this.readonly = !!options?.readonly;
  }
};
var ConsoleStdout = class _ConsoleStdout extends Fd {
  fd_filestat_get() {
    const filestat = new Filestat(this.ino, FILETYPE_CHARACTER_DEVICE, BigInt(0));
    return { ret: 0, filestat };
  }
  fd_fdstat_get() {
    const fdstat = new Fdstat(FILETYPE_CHARACTER_DEVICE, 0);
    fdstat.fs_rights_base = BigInt(RIGHTS_FD_WRITE);
    return { ret: 0, fdstat };
  }
  fd_write(data) {
    this.write(data);
    return { ret: 0, nwritten: data.byteLength };
  }
  static lineBuffered(write) {
    const dec = new TextDecoder("utf-8", { fatal: false });
    let line_buf = "";
    return new _ConsoleStdout((buffer) => {
      line_buf += dec.decode(buffer, { stream: true });
      const lines = line_buf.split("\n");
      for (const [i, line] of lines.entries()) {
        if (i < lines.length - 1) {
          write(line);
        } else {
          line_buf = line;
        }
      }
    });
  }
  constructor(write) {
    super();
    this.ino = Inode.issue_ino();
    this.write = write;
  }
};

// wasi/fs.ts
var File2 = class extends Inode {
  handle;
  readonly;
  // FIXME needs a close() method to be called after start() to release the underlying handle
  constructor(handle, options) {
    super();
    this.handle = handle;
    this.readonly = !!options?.readonly;
  }
  path_open(oflags, fs_rights_base, fd_flags) {
    if (this.readonly && (fs_rights_base & BigInt(wasi_defs_exports.RIGHTS_FD_WRITE)) == BigInt(wasi_defs_exports.RIGHTS_FD_WRITE)) {
      return { ret: wasi_defs_exports.ERRNO_PERM, fd_obj: null };
    }
    if ((oflags & wasi_defs_exports.OFLAGS_TRUNC) == wasi_defs_exports.OFLAGS_TRUNC) {
      if (this.readonly) return { ret: wasi_defs_exports.ERRNO_PERM, fd_obj: null };
      this.handle.truncate(0);
    }
    const file = new OpenFile2(this);
    if (fd_flags & wasi_defs_exports.FDFLAGS_APPEND) file.fd_seek(0n, wasi_defs_exports.WHENCE_END);
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, fd_obj: file };
  }
  get size() {
    return BigInt(this.handle.getSize());
  }
  stat() {
    return new wasi_defs_exports.Filestat(this.ino, wasi_defs_exports.FILETYPE_REGULAR_FILE, this.size);
  }
};
var OpenFile2 = class extends Fd {
  file;
  position = 0n;
  ino;
  constructor(file) {
    super();
    this.file = file;
    this.ino = Inode.issue_ino();
    this.file.handle.open();
  }
  fd_allocate(offset, len) {
    if (BigInt(this.file.handle.getSize()) > offset + len) {
    } else {
      this.file.handle.truncate(Number(offset + len));
    }
    return wasi_defs_exports.ERRNO_SUCCESS;
  }
  fd_fdstat_get() {
    const size = this.file.handle.getSize();
    const fdstat = new wasi_defs_exports.Fdstat(size > 0 ? wasi_defs_exports.FILETYPE_REGULAR_FILE : wasi_defs_exports.FILETYPE_CHARACTER_DEVICE, 0);
    if (!this.file.readonly) {
      fdstat.fs_rights_base = BigInt(wasi_defs_exports.RIGHTS_FD_WRITE);
    }
    return { ret: 0, fdstat };
  }
  fd_filestat_get() {
    const size = this.file.handle.getSize();
    return {
      ret: 0,
      filestat: new wasi_defs_exports.Filestat(
        this.ino,
        size > 0 ? wasi_defs_exports.FILETYPE_REGULAR_FILE : wasi_defs_exports.FILETYPE_CHARACTER_DEVICE,
        BigInt(size)
      )
    };
  }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  fd_filestat_set_size(size) {
    this.file.handle.truncate(Number(size));
    return wasi_defs_exports.ERRNO_SUCCESS;
  }
  fd_read(size) {
    const buf = new Uint8Array(size);
    const n = this.file.handle.read(buf, { at: Number(this.position) });
    this.position += BigInt(n);
    return { ret: 0, data: buf.slice(0, n) };
  }
  fd_seek(offset, whence) {
    let calculated_offset;
    switch (whence) {
      case wasi_defs_exports.WHENCE_SET:
        calculated_offset = BigInt(offset);
        break;
      case wasi_defs_exports.WHENCE_CUR:
        calculated_offset = this.position + BigInt(offset);
        break;
      case wasi_defs_exports.WHENCE_END:
        calculated_offset = BigInt(this.file.handle.getSize()) + BigInt(offset);
        break;
      default:
        return { ret: wasi_defs_exports.ERRNO_INVAL, offset: 0n };
    }
    if (calculated_offset < 0) {
      return { ret: wasi_defs_exports.ERRNO_INVAL, offset: 0n };
    }
    this.position = calculated_offset;
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, offset: this.position };
  }
  fd_write(data) {
    if (this.file.readonly) return { ret: wasi_defs_exports.ERRNO_BADF, nwritten: 0 };
    const n = this.file.handle.write(data, { at: Number(this.position) });
    this.position += BigInt(n);
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, nwritten: n };
  }
  fd_sync() {
    this.file.handle.flush();
    return wasi_defs_exports.ERRNO_SUCCESS;
  }
};
var OpenDirectory2 = class extends Fd {
  dir;
  constructor(dir) {
    super();
    this.dir = dir;
  }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  fd_seek(offset, whence) {
    return { ret: wasi_defs_exports.ERRNO_BADF, offset: 0n };
  }
  fd_tell() {
    return { ret: wasi_defs_exports.ERRNO_BADF, offset: 0n };
  }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  fd_allocate(offset, len) {
    return wasi_defs_exports.ERRNO_BADF;
  }
  fd_fdstat_get() {
    return { ret: 0, fdstat: new wasi_defs_exports.Fdstat(wasi_defs_exports.FILETYPE_DIRECTORY, 0) };
  }
  fd_readdir_single(cookie) {
    if (cookie == 0n) {
      return {
        ret: wasi_defs_exports.ERRNO_SUCCESS,
        dirent: new wasi_defs_exports.Dirent(1n, this.dir.ino, ".", wasi_defs_exports.FILETYPE_DIRECTORY)
      };
    } else if (cookie == 1n) {
      return {
        ret: wasi_defs_exports.ERRNO_SUCCESS,
        dirent: new wasi_defs_exports.Dirent(
          2n,
          this.dir.parent_ino(),
          "..",
          wasi_defs_exports.FILETYPE_DIRECTORY
        )
      };
    }
    if (cookie >= BigInt(this.dir.contents.size) + 2n) {
      return { ret: 0, dirent: null };
    }
    const [name, entry] = Array.from(this.dir.contents.entries())[Number(cookie - 2n)];
    return {
      ret: 0,
      dirent: new wasi_defs_exports.Dirent(
        cookie + 1n,
        entry.ino,
        name,
        entry.stat().filetype
      )
    };
  }
  path_filestat_get(flags, path_str) {
    const { ret: path_err, path } = Path.from(path_str);
    if (path == null) {
      return { ret: path_err, filestat: null };
    }
    const { ret, entry } = this.dir.get_entry_for_path(path);
    if (entry == null) {
      return { ret, filestat: null };
    }
    return { ret: 0, filestat: entry.stat() };
  }
  path_lookup(path_str, dirflags) {
    const { ret: path_ret, path } = Path.from(path_str);
    if (path == null) {
      return { ret: path_ret, inode_obj: null };
    }
    const { ret, entry } = this.dir.get_entry_for_path(path);
    if (entry == null) {
      return { ret, inode_obj: null };
    }
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, inode_obj: entry };
  }
  path_open(dirflags, path_str, oflags, fs_rights_base, fs_rights_inheriting, fd_flags) {
    const { ret: path_ret, path } = Path.from(path_str);
    if (path == null) {
      return { ret: path_ret, fd_obj: null };
    }
    let { ret, entry } = this.dir.get_entry_for_path(path);
    if (entry == null) {
      if (ret != wasi_defs_exports.ERRNO_NOENT) {
        return { ret, fd_obj: null };
      }
      if ((oflags & wasi_defs_exports.OFLAGS_CREAT) == wasi_defs_exports.OFLAGS_CREAT) {
        const { ret: ret2, entry: new_entry } = this.dir.create_entry_for_path(
          path_str,
          (oflags & wasi_defs_exports.OFLAGS_DIRECTORY) == wasi_defs_exports.OFLAGS_DIRECTORY
        );
        if (new_entry == null) {
          return { ret: ret2, fd_obj: null };
        }
        entry = new_entry;
      } else {
        return { ret: wasi_defs_exports.ERRNO_NOENT, fd_obj: null };
      }
    } else if ((oflags & wasi_defs_exports.OFLAGS_EXCL) == wasi_defs_exports.OFLAGS_EXCL) {
      return { ret: wasi_defs_exports.ERRNO_EXIST, fd_obj: null };
    }
    if ((oflags & wasi_defs_exports.OFLAGS_DIRECTORY) == wasi_defs_exports.OFLAGS_DIRECTORY && entry.stat().filetype !== wasi_defs_exports.FILETYPE_DIRECTORY) {
      return { ret: wasi_defs_exports.ERRNO_NOTDIR, fd_obj: null };
    }
    return entry.path_open(oflags, fs_rights_base, fd_flags);
  }
  path_create_directory(path) {
    return this.path_open(
      0,
      path,
      wasi_defs_exports.OFLAGS_CREAT | wasi_defs_exports.OFLAGS_DIRECTORY,
      0n,
      0n,
      0
    ).ret;
  }
  path_link(path_str, inode, allow_dir) {
    const { ret: path_ret, path } = Path.from(path_str);
    if (path == null) {
      return path_ret;
    }
    if (path.is_dir) {
      return wasi_defs_exports.ERRNO_NOENT;
    }
    const {
      ret: parent_ret,
      parent_entry,
      filename,
      entry
    } = this.dir.get_parent_dir_and_entry_for_path(path, true);
    if (parent_entry == null || filename == null) {
      return parent_ret;
    }
    if (entry != null) {
      const source_is_dir = inode.stat().filetype == wasi_defs_exports.FILETYPE_DIRECTORY;
      const target_is_dir = entry.stat().filetype == wasi_defs_exports.FILETYPE_DIRECTORY;
      if (source_is_dir && target_is_dir) {
        if (allow_dir && entry instanceof Directory2) {
          if (entry.contents.size == 0) {
          } else {
            return wasi_defs_exports.ERRNO_NOTEMPTY;
          }
        } else {
          return wasi_defs_exports.ERRNO_EXIST;
        }
      } else if (source_is_dir && !target_is_dir) {
        return wasi_defs_exports.ERRNO_NOTDIR;
      } else if (!source_is_dir && target_is_dir) {
        return wasi_defs_exports.ERRNO_ISDIR;
      } else if (inode.stat().filetype == wasi_defs_exports.FILETYPE_REGULAR_FILE && entry.stat().filetype == wasi_defs_exports.FILETYPE_REGULAR_FILE) {
      } else {
        return wasi_defs_exports.ERRNO_EXIST;
      }
    }
    if (!allow_dir && inode.stat().filetype == wasi_defs_exports.FILETYPE_DIRECTORY) {
      return wasi_defs_exports.ERRNO_PERM;
    }
    parent_entry.createLink(filename, inode);
    return wasi_defs_exports.ERRNO_SUCCESS;
  }
  path_unlink(path_str) {
    const { ret: path_ret, path } = Path.from(path_str);
    if (path == null) {
      return { ret: path_ret, inode_obj: null };
    }
    const {
      ret: parent_ret,
      parent_entry,
      filename,
      entry
    } = this.dir.get_parent_dir_and_entry_for_path(path, true);
    if (parent_entry == null || filename == null) {
      return { ret: parent_ret, inode_obj: null };
    }
    if (entry == null) {
      return { ret: wasi_defs_exports.ERRNO_NOENT, inode_obj: null };
    }
    parent_entry.removeEntry(filename);
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, inode_obj: entry };
  }
  path_unlink_file(path_str) {
    const { ret: path_ret, path } = Path.from(path_str);
    if (path == null) {
      return path_ret;
    }
    const {
      ret: parent_ret,
      parent_entry,
      filename,
      entry
    } = this.dir.get_parent_dir_and_entry_for_path(path, false);
    if (parent_entry == null || filename == null || entry == null) {
      return parent_ret;
    }
    if (entry.stat().filetype === wasi_defs_exports.FILETYPE_DIRECTORY) {
      return wasi_defs_exports.ERRNO_ISDIR;
    }
    parent_entry.removeEntry(filename);
    return wasi_defs_exports.ERRNO_SUCCESS;
  }
  path_remove_directory(path_str) {
    const { ret: path_ret, path } = Path.from(path_str);
    if (path == null) {
      return path_ret;
    }
    const {
      ret: parent_ret,
      parent_entry,
      filename,
      entry
    } = this.dir.get_parent_dir_and_entry_for_path(path, false);
    if (parent_entry == null || filename == null || entry == null) {
      return parent_ret;
    }
    if (!(entry instanceof Directory2) || entry.stat().filetype !== wasi_defs_exports.FILETYPE_DIRECTORY) {
      return wasi_defs_exports.ERRNO_NOTDIR;
    }
    entry.syncEntries();
    if (entry.contents.size !== 0) {
      return wasi_defs_exports.ERRNO_NOTEMPTY;
    }
    if (!parent_entry.removeEntry(filename)) {
      return wasi_defs_exports.ERRNO_NOENT;
    }
    return wasi_defs_exports.ERRNO_SUCCESS;
  }
  fd_filestat_get() {
    return { ret: 0, filestat: this.dir.stat() };
  }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  fd_filestat_set_size(size) {
    return wasi_defs_exports.ERRNO_BADF;
  }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  fd_read(size) {
    return { ret: wasi_defs_exports.ERRNO_BADF, data: new Uint8Array() };
  }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  fd_pread(size, offset) {
    return { ret: wasi_defs_exports.ERRNO_BADF, data: new Uint8Array() };
  }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  fd_write(data) {
    return { ret: wasi_defs_exports.ERRNO_BADF, nwritten: 0 };
  }
  fd_pwrite(data, offset) {
    return { ret: wasi_defs_exports.ERRNO_BADF, nwritten: 0 };
  }
};
var PreopenDirectory2 = class extends OpenDirectory2 {
  prestat_name;
  constructor(name, dir) {
    super(dir);
    this.prestat_name = name;
  }
  fd_prestat_get() {
    return {
      ret: 0,
      prestat: wasi_defs_exports.Prestat.dir(this.prestat_name)
    };
  }
};
var Path = class _Path {
  parts = [];
  is_dir = false;
  static from(path) {
    const self = new _Path();
    self.is_dir = path.endsWith("/");
    if (path.startsWith("/")) {
      return { ret: wasi_defs_exports.ERRNO_NOTCAPABLE, path: null };
    }
    if (path.includes("\0")) {
      return { ret: wasi_defs_exports.ERRNO_INVAL, path: null };
    }
    for (const component of path.split("/")) {
      if (component === "" || component === ".") {
        continue;
      }
      if (component === "..") {
        if (self.parts.pop() == void 0) {
          return { ret: wasi_defs_exports.ERRNO_NOTCAPABLE, path: null };
        }
        continue;
      }
      self.parts.push(component);
    }
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, path: self };
  }
  to_path_string() {
    let s = this.parts.join("/");
    if (this.is_dir) {
      s += "/";
    }
    return s;
  }
};
var Directory2 = class _Directory extends Inode {
  contents;
  parent = null;
  handle;
  constructor(handle) {
    super();
    this.handle = handle;
  }
  syncEntries() {
    this.contents = this.handle.readDir();
    for (const entry of this.contents.values()) {
      if (entry instanceof _Directory) {
        entry.parent = this;
      }
    }
  }
  removeEntry(name) {
    if (this.handle.removeEntry(name)) {
      return this.contents.delete(name);
    }
    return false;
  }
  createLink(name, entry) {
    if (this.handle.createLink(name, entry)) {
      this.contents.set(name, entry);
    }
  }
  createFile(name, entry) {
    if (this.handle.createFile(name, entry)) {
      this.contents.set(name, entry);
    }
  }
  createDirectory(name, entry) {
    if (this.handle.createDirectory(name, entry)) {
      this.contents.set(name, entry);
    }
  }
  parent_ino() {
    if (this.parent == null) {
      return Inode.root_ino();
    }
    return this.parent.ino;
  }
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  path_open(oflags, fs_rights_base, fd_flags) {
    this.syncEntries();
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, fd_obj: new OpenDirectory2(this) };
  }
  stat() {
    return new wasi_defs_exports.Filestat(this.ino, wasi_defs_exports.FILETYPE_DIRECTORY, 0n);
  }
  get_entry_for_path(path) {
    let entry = this;
    for (const component of path.parts) {
      if (!(entry instanceof _Directory)) {
        return { ret: wasi_defs_exports.ERRNO_NOTDIR, entry: null };
      }
      entry.syncEntries();
      const child = entry.contents.get(component);
      if (child !== void 0) {
        entry = child;
      } else {
        return { ret: wasi_defs_exports.ERRNO_NOENT, entry: null };
      }
    }
    if (path.is_dir) {
      if (entry.stat().filetype != wasi_defs_exports.FILETYPE_DIRECTORY) {
        return { ret: wasi_defs_exports.ERRNO_NOTDIR, entry: null };
      }
    }
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, entry };
  }
  get_parent_dir_and_entry_for_path(path, allow_undefined) {
    const filename = path.parts.pop();
    if (filename === void 0) {
      return {
        ret: wasi_defs_exports.ERRNO_INVAL,
        parent_entry: null,
        filename: null,
        entry: null
      };
    }
    const { ret: entry_ret, entry: parent_entry } = this.get_entry_for_path(path);
    if (parent_entry == null) {
      return {
        ret: entry_ret,
        parent_entry: null,
        filename: null,
        entry: null
      };
    }
    if (!(parent_entry instanceof _Directory)) {
      return {
        ret: wasi_defs_exports.ERRNO_NOTDIR,
        parent_entry: null,
        filename: null,
        entry: null
      };
    }
    parent_entry.syncEntries();
    const entry = parent_entry.contents.get(filename);
    if (entry === void 0) {
      if (!allow_undefined) {
        return {
          ret: wasi_defs_exports.ERRNO_NOENT,
          parent_entry: null,
          filename: null,
          entry: null
        };
      } else {
        return { ret: wasi_defs_exports.ERRNO_SUCCESS, parent_entry, filename, entry: null };
      }
    }
    if (path.is_dir) {
      if (entry.stat().filetype != wasi_defs_exports.FILETYPE_DIRECTORY) {
        return {
          ret: wasi_defs_exports.ERRNO_NOTDIR,
          parent_entry: null,
          filename: null,
          entry: null
        };
      }
    }
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, parent_entry, filename, entry };
  }
  create_entry_for_path(path_str, is_dir) {
    const { ret: path_ret, path } = Path.from(path_str);
    if (path == null) {
      return { ret: path_ret, entry: null };
    }
    let {
      // eslint-disable-next-line prefer-const
      ret: parent_ret,
      // eslint-disable-next-line prefer-const
      parent_entry,
      // eslint-disable-next-line prefer-const
      filename,
      entry
    } = this.get_parent_dir_and_entry_for_path(path, true);
    if (parent_entry == null || filename == null) {
      return { ret: parent_ret, entry: null };
    }
    if (entry != null) {
      return { ret: wasi_defs_exports.ERRNO_EXIST, entry: null };
    }
    let new_child;
    if (!is_dir) {
      new_child = parent_entry.handle.newEntry(filename, false);
      parent_entry.createFile(filename, new_child);
    } else {
      new_child = parent_entry.handle.newEntry(filename, true);
      parent_entry.createDirectory(filename, new_child);
    }
    entry = new_child;
    return { ret: wasi_defs_exports.ERRNO_SUCCESS, entry };
  }
};

// wasi/wanix.ts
var WanixHandle = class {
  caller;
  path;
  constructor(caller, path) {
    this.caller = caller;
    this.path = path;
  }
  subpath(path) {
    if (this.path === ".") {
      return path;
    }
    return [this.path, path].join("/");
  }
};
var FileHandle = class extends WanixHandle {
  fd;
  open() {
    this.fd = this.caller.call("path_open", { path: this.path });
  }
  close() {
    this.caller.call("fd_close", { fd: this.fd });
  }
  flush() {
    this.caller.call("fd_flush", { fd: this.fd });
  }
  read(buffer, options) {
    let at = 0;
    if (options?.at) {
      at = options.at;
    }
    const count = buffer.byteLength;
    const data = this.caller.call("fd_read", { fd: this.fd, count, at });
    let writeBuffer3;
    if (buffer instanceof ArrayBuffer) {
      writeBuffer3 = new Uint8Array(buffer);
    } else if (buffer instanceof Uint8Array) {
      writeBuffer3 = buffer;
    } else {
      throw new Error("Buffer must be ArrayBuffer or Uint8Array");
    }
    writeBuffer3.set(data, 0);
    return data.length;
  }
  write(buffer, options) {
    let at = 0;
    if (options?.at) {
      at = options.at;
    }
    const data = new Uint8Array(buffer);
    return this.caller.call("fd_write", { fd: this.fd, data, at });
  }
  truncate(to) {
    this.caller.call("path_truncate", { path: this.path, to });
  }
  getSize() {
    return this.caller.call("path_size", { path: this.path });
  }
};
var DirectoryHandle = class _DirectoryHandle extends WanixHandle {
  dirCache;
  lastReadDir;
  newEntry(name, isDir) {
    if (isDir) {
      const handle = new _DirectoryHandle(this.caller, this.subpath(name));
      return new Directory2(handle);
    } else {
      const handle = new FileHandle(this.caller, this.subpath(name));
      return new File2(handle);
    }
  }
  readDir() {
    if (performance.now() - this.lastReadDir < 1e3) {
      return this.dirCache;
    }
    this.lastReadDir = performance.now();
    const m = /* @__PURE__ */ new Map();
    const entries = this.caller.call("path_readdir", { path: this.path }) || [];
    for (const entry of entries) {
      let isDir = false;
      let name = entry;
      if (name.slice(-1) === "/") {
        isDir = true;
        name = name.slice(0, -1);
      }
      m.set(name, this.newEntry(name, isDir));
    }
    this.dirCache = m;
    return m;
  }
  removeEntry(name) {
    return this.caller.call("path_remove", { path: this.subpath(name) });
  }
  createLink(name, entry) {
    return false;
  }
  createFile(name, entry) {
    return this.caller.call("path_touch", { path: this.subpath(name) });
  }
  createDirectory(name, entry) {
    return this.caller.call("path_mkdir", { path: this.subpath(name) });
  }
};

// wasi/empty.ts
var EmptyFile = class extends File {
  constructor() {
    super([]);
  }
};
var OpenEmptyFile = class extends OpenFile {
  constructor() {
    super(new EmptyFile());
  }
};

// wasi/poll-oneoff.ts
function applyPatchPollOneoff(self) {
  self.wasiImport.poll_oneoff = (inPtr, outPtr, nsubscriptions, sizeOutPtr) => {
    if (nsubscriptions < 0) {
      return wasi_defs_exports.ERRNO_INVAL;
    }
    const size_subscription = 48;
    const subscriptions = new DataView(
      self.inst.exports.memory.buffer,
      inPtr,
      nsubscriptions * size_subscription
    );
    const size_event = 32;
    const events = new DataView(
      self.inst.exports.memory.buffer,
      outPtr,
      nsubscriptions * size_event
    );
    for (let i = 0; i < nsubscriptions; ++i) {
      let assertOpenFileAvailable = function() {
        const fd = subscriptions.getUint32(
          i * size_subscription + subscription_u_offset + subscription_u_tag_size,
          true
        );
        const openFile = self.fds[fd];
        if (!(openFile instanceof OpenFile2)) {
          throw new Error(`FD#${fd} cannot be polled!`);
        }
        return openFile;
      }, setEventFdReadWrite = function(size) {
        events.setUint16(
          i * size_event + event_type_offset,
          wasi_defs_exports.EVENTTYPE_FD_READ,
          true
        );
        events.setBigUint64(
          i * size_event + event_fd_readwrite_nbytes_offset,
          size,
          true
        );
        events.setUint16(
          i * size_event + event_fd_readwrite_flags_offset,
          0,
          true
        );
      };
      const subscription_userdata_offset = 0;
      const userdata = subscriptions.getBigUint64(
        i * size_subscription + subscription_userdata_offset,
        true
      );
      const subscription_u_offset = 8;
      const subscription_u_tag = subscriptions.getUint8(
        i * size_subscription + subscription_u_offset
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
        true
      );
      events.setUint32(
        i * size_event + event_error_offset,
        wasi_defs_exports.ERRNO_SUCCESS,
        true
      );
      switch (subscription_u_tag) {
        case wasi_defs_exports.EVENTTYPE_CLOCK:
          events.setUint16(
            i * size_event + event_type_offset,
            wasi_defs_exports.EVENTTYPE_CLOCK,
            true
          );
          break;
        case wasi_defs_exports.EVENTTYPE_FD_READ:
          const fileR = assertOpenFileAvailable();
          setEventFdReadWrite(fileR.file.size);
          break;
        case wasi_defs_exports.EVENTTYPE_FD_WRITE:
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
      size_size
    );
    outNSize.setUint32(0, nsubscriptions, true);
    return wasi_defs_exports.ERRNO_SUCCESS;
  };
}

// node_modules/@progrium/duplex/esm/codec/json.js
(function() {
  if (typeof global !== "undefined" && !global.TextEncoder) {
    const { TextEncoder: TextEncoder2, TextDecoder: TextDecoder2 } = __require("util");
    global.TextEncoder = TextEncoder2;
    global.TextDecoder = TextDecoder2;
  }
})();

// node_modules/@progrium/duplex/esm/vnd/cbor-x-1.4.1/decode.js
var decoder2;
try {
  decoder2 = new TextDecoder();
} catch (error) {
}
var src2;
var srcEnd2;
var position3 = 0;
var EMPTY_ARRAY2 = [];
var LEGACY_RECORD_INLINE_ID2 = 105;
var RECORD_DEFINITIONS_ID2 = 57342;
var RECORD_INLINE_ID2 = 57343;
var BUNDLED_STRINGS_ID2 = 57337;
var PACKED_REFERENCE_TAG_ID2 = 6;
var STOP_CODE2 = {};
var strings2 = EMPTY_ARRAY2;
var stringPosition2 = 0;
var currentDecoder2 = {};
var currentStructures2;
var srcString2;
var srcStringStart2 = 0;
var srcStringEnd2 = 0;
var bundledStrings3;
var referenceMap2;
var currentExtensions2 = [];
var currentExtensionRanges2 = [];
var packedValues2;
var dataView2;
var restoreMapsAsObject2;
var defaultOptions2 = {
  useRecords: false,
  mapsAsObjects: true
};
var sequentialMode2 = false;
var Decoder2 = class _Decoder {
  constructor(options) {
    if (options) {
      if ((options.keyMap || options._keyMap) && !options.useRecords) {
        options.useRecords = false;
        options.mapsAsObjects = true;
      }
      if (options.useRecords === false && options.mapsAsObjects === void 0)
        options.mapsAsObjects = true;
      if (options.getStructures)
        options.getShared = options.getStructures;
      if (options.getShared && !options.structures)
        (options.structures = []).uninitialized = true;
      if (options.keyMap) {
        this.mapKey = /* @__PURE__ */ new Map();
        for (let [k, v] of Object.entries(options.keyMap))
          this.mapKey.set(v, k);
      }
    }
    Object.assign(this, options);
  }
  /*
  decodeKey(key) {
      return this.keyMap
          ? Object.keys(this.keyMap)[Object.values(this.keyMap).indexOf(key)] || key
          : key
  }
  */
  decodeKey(key) {
    return this.keyMap ? this.mapKey.get(key) || key : key;
  }
  encodeKey(key) {
    return this.keyMap && this.keyMap.hasOwnProperty(key) ? this.keyMap[key] : key;
  }
  encodeKeys(rec) {
    if (!this._keyMap)
      return rec;
    let map = /* @__PURE__ */ new Map();
    for (let [k, v] of Object.entries(rec))
      map.set(this._keyMap.hasOwnProperty(k) ? this._keyMap[k] : k, v);
    return map;
  }
  decodeKeys(map) {
    if (!this._keyMap || map.constructor.name != "Map")
      return map;
    if (!this._mapKey) {
      this._mapKey = /* @__PURE__ */ new Map();
      for (let [k, v] of Object.entries(this._keyMap))
        this._mapKey.set(v, k);
    }
    let res = {};
    map.forEach((v, k) => res[safeKey2(this._mapKey.has(k) ? this._mapKey.get(k) : k)] = v);
    return res;
  }
  mapDecode(source, end) {
    let res = this.decode(source);
    if (this._keyMap) {
      switch (res.constructor.name) {
        case "Array":
          return res.map((r) => this.decodeKeys(r));
      }
    }
    return res;
  }
  decode(source, end) {
    if (src2) {
      return saveState2(() => {
        clearSource2();
        return this ? this.decode(source, end) : _Decoder.prototype.decode.call(defaultOptions2, source, end);
      });
    }
    srcEnd2 = end > -1 ? end : source.length;
    position3 = 0;
    stringPosition2 = 0;
    srcStringEnd2 = 0;
    srcString2 = null;
    strings2 = EMPTY_ARRAY2;
    bundledStrings3 = null;
    src2 = source;
    try {
      dataView2 = source.dataView || (source.dataView = new DataView(source.buffer, source.byteOffset, source.byteLength));
    } catch (error) {
      src2 = null;
      if (source instanceof Uint8Array)
        throw error;
      throw new Error("Source must be a Uint8Array or Buffer but was a " + (source && typeof source == "object" ? source.constructor.name : typeof source));
    }
    if (this instanceof _Decoder) {
      currentDecoder2 = this;
      packedValues2 = this.sharedValues && (this.pack ? new Array(this.maxPrivatePackedValues || 16).concat(this.sharedValues) : this.sharedValues);
      if (this.structures) {
        currentStructures2 = this.structures;
        return checkedRead2();
      } else if (!currentStructures2 || currentStructures2.length > 0) {
        currentStructures2 = [];
      }
    } else {
      currentDecoder2 = defaultOptions2;
      if (!currentStructures2 || currentStructures2.length > 0)
        currentStructures2 = [];
      packedValues2 = null;
    }
    return checkedRead2();
  }
  decodeMultiple(source, forEach) {
    let values, lastPosition = 0;
    try {
      let size = source.length;
      sequentialMode2 = true;
      let value = this ? this.decode(source, size) : defaultDecoder2.decode(source, size);
      if (forEach) {
        if (forEach(value) === false) {
          return;
        }
        while (position3 < size) {
          lastPosition = position3;
          if (forEach(checkedRead2()) === false) {
            return;
          }
        }
      } else {
        values = [value];
        while (position3 < size) {
          lastPosition = position3;
          values.push(checkedRead2());
        }
        return values;
      }
    } catch (error) {
      error.lastPosition = lastPosition;
      error.values = values;
      throw error;
    } finally {
      sequentialMode2 = false;
      clearSource2();
    }
  }
};
function checkedRead2() {
  try {
    let result = read2();
    if (bundledStrings3) {
      if (position3 >= bundledStrings3.postBundlePosition) {
        let error = new Error("Unexpected bundle position");
        error.incomplete = true;
        throw error;
      }
      position3 = bundledStrings3.postBundlePosition;
      bundledStrings3 = null;
    }
    if (position3 == srcEnd2) {
      currentStructures2 = null;
      src2 = null;
      if (referenceMap2)
        referenceMap2 = null;
    } else if (position3 > srcEnd2) {
      let error = new Error("Unexpected end of CBOR data");
      error.incomplete = true;
      throw error;
    } else if (!sequentialMode2) {
      throw new Error("Data read, but end of buffer not reached");
    }
    return result;
  } catch (error) {
    clearSource2();
    if (error instanceof RangeError || error.message.startsWith("Unexpected end of buffer")) {
      error.incomplete = true;
    }
    throw error;
  }
}
function read2() {
  let token = src2[position3++];
  let majorType = token >> 5;
  token = token & 31;
  if (token > 23) {
    switch (token) {
      case 24:
        token = src2[position3++];
        break;
      case 25:
        if (majorType == 7) {
          return getFloat162();
        }
        token = dataView2.getUint16(position3);
        position3 += 2;
        break;
      case 26:
        if (majorType == 7) {
          let value = dataView2.getFloat32(position3);
          if (currentDecoder2.useFloat32 > 2) {
            let multiplier = mult102[(src2[position3] & 127) << 1 | src2[position3 + 1] >> 7];
            position3 += 4;
            return (multiplier * value + (value > 0 ? 0.5 : -0.5) >> 0) / multiplier;
          }
          position3 += 4;
          return value;
        }
        token = dataView2.getUint32(position3);
        position3 += 4;
        break;
      case 27:
        if (majorType == 7) {
          let value = dataView2.getFloat64(position3);
          position3 += 8;
          return value;
        }
        if (majorType > 1) {
          if (dataView2.getUint32(position3) > 0)
            throw new Error("JavaScript does not support arrays, maps, or strings with length over 4294967295");
          token = dataView2.getUint32(position3 + 4);
        } else if (currentDecoder2.int64AsNumber) {
          token = dataView2.getUint32(position3) * 4294967296;
          token += dataView2.getUint32(position3 + 4);
        } else
          token = dataView2.getBigUint64(position3);
        position3 += 8;
        break;
      case 31:
        switch (majorType) {
          case 2:
          // byte string
          case 3:
            throw new Error("Indefinite length not supported for byte or text strings");
          case 4:
            let array = [];
            let value, i = 0;
            while ((value = read2()) != STOP_CODE2) {
              array[i++] = value;
            }
            return majorType == 4 ? array : majorType == 3 ? array.join("") : Buffer.concat(array);
          case 5:
            let key;
            if (currentDecoder2.mapsAsObjects) {
              let object = {};
              if (currentDecoder2.keyMap)
                while ((key = read2()) != STOP_CODE2)
                  object[safeKey2(currentDecoder2.decodeKey(key))] = read2();
              else
                while ((key = read2()) != STOP_CODE2)
                  object[safeKey2(key)] = read2();
              return object;
            } else {
              if (restoreMapsAsObject2) {
                currentDecoder2.mapsAsObjects = true;
                restoreMapsAsObject2 = false;
              }
              let map = /* @__PURE__ */ new Map();
              if (currentDecoder2.keyMap)
                while ((key = read2()) != STOP_CODE2)
                  map.set(currentDecoder2.decodeKey(key), read2());
              else
                while ((key = read2()) != STOP_CODE2)
                  map.set(key, read2());
              return map;
            }
          case 7:
            return STOP_CODE2;
          default:
            throw new Error("Invalid major type for indefinite length " + majorType);
        }
      default:
        throw new Error("Unknown token " + token);
    }
  }
  switch (majorType) {
    case 0:
      return token;
    case 1:
      return ~token;
    case 2:
      return readBin2(token);
    case 3:
      if (srcStringEnd2 >= position3) {
        return srcString2.slice(position3 - srcStringStart2, (position3 += token) - srcStringStart2);
      }
      if (srcStringEnd2 == 0 && srcEnd2 < 140 && token < 32) {
        let string = token < 16 ? shortStringInJS2(token) : longStringInJS2(token);
        if (string != null)
          return string;
      }
      return readFixedString2(token);
    case 4:
      let array = new Array(token);
      for (let i = 0; i < token; i++)
        array[i] = read2();
      return array;
    case 5:
      if (currentDecoder2.mapsAsObjects) {
        let object = {};
        if (currentDecoder2.keyMap)
          for (let i = 0; i < token; i++)
            object[safeKey2(currentDecoder2.decodeKey(read2()))] = read2();
        else
          for (let i = 0; i < token; i++)
            object[safeKey2(read2())] = read2();
        return object;
      } else {
        if (restoreMapsAsObject2) {
          currentDecoder2.mapsAsObjects = true;
          restoreMapsAsObject2 = false;
        }
        let map = /* @__PURE__ */ new Map();
        if (currentDecoder2.keyMap)
          for (let i = 0; i < token; i++)
            map.set(currentDecoder2.decodeKey(read2()), read2());
        else
          for (let i = 0; i < token; i++)
            map.set(read2(), read2());
        return map;
      }
    case 6:
      if (token >= BUNDLED_STRINGS_ID2) {
        let structure = currentStructures2[token & 8191];
        if (structure) {
          if (!structure.read)
            structure.read = createStructureReader2(structure);
          return structure.read();
        }
        if (token < 65536) {
          if (token == RECORD_INLINE_ID2) {
            let length = readJustLength2();
            let id = read2();
            let structure2 = read2();
            recordDefinition2(id, structure2);
            let object = {};
            if (currentDecoder2.keyMap)
              for (let i = 2; i < length; i++) {
                let key = currentDecoder2.decodeKey(structure2[i - 2]);
                object[safeKey2(key)] = read2();
              }
            else
              for (let i = 2; i < length; i++) {
                let key = structure2[i - 2];
                object[safeKey2(key)] = read2();
              }
            return object;
          } else if (token == RECORD_DEFINITIONS_ID2) {
            let length = readJustLength2();
            let id = read2();
            for (let i = 2; i < length; i++) {
              recordDefinition2(id++, read2());
            }
            return read2();
          } else if (token == BUNDLED_STRINGS_ID2) {
            return readBundleExt2();
          }
          if (currentDecoder2.getShared) {
            loadShared2();
            structure = currentStructures2[token & 8191];
            if (structure) {
              if (!structure.read)
                structure.read = createStructureReader2(structure);
              return structure.read();
            }
          }
        }
      }
      let extension = currentExtensions2[token];
      if (extension) {
        if (extension.handlesRead)
          return extension(read2);
        else
          return extension(read2());
      } else {
        let input = read2();
        for (let i = 0; i < currentExtensionRanges2.length; i++) {
          let value = currentExtensionRanges2[i](token, input);
          if (value !== void 0)
            return value;
        }
        return new Tag2(input, token);
      }
    case 7:
      switch (token) {
        case 20:
          return false;
        case 21:
          return true;
        case 22:
          return null;
        case 23:
          return;
        // undefined
        case 31:
        default:
          let packedValue = (packedValues2 || getPackedValues2())[token];
          if (packedValue !== void 0)
            return packedValue;
          throw new Error("Unknown token " + token);
      }
    default:
      if (isNaN(token)) {
        let error = new Error("Unexpected end of CBOR data");
        error.incomplete = true;
        throw error;
      }
      throw new Error("Unknown CBOR token " + token);
  }
}
var validName2 = /^[a-zA-Z_$][a-zA-Z\d_$]*$/;
function createStructureReader2(structure) {
  function readObject() {
    let length = src2[position3++];
    length = length & 31;
    if (length > 23) {
      switch (length) {
        case 24:
          length = src2[position3++];
          break;
        case 25:
          length = dataView2.getUint16(position3);
          position3 += 2;
          break;
        case 26:
          length = dataView2.getUint32(position3);
          position3 += 4;
          break;
        default:
          throw new Error("Expected array header, but got " + src2[position3 - 1]);
      }
    }
    let compiledReader = this.compiledReader;
    while (compiledReader) {
      if (compiledReader.propertyCount === length)
        return compiledReader(read2);
      compiledReader = compiledReader.next;
    }
    if (this.slowReads++ >= 3) {
      let array = this.length == length ? this : this.slice(0, length);
      compiledReader = currentDecoder2.keyMap ? new Function("r", "return {" + array.map((k) => currentDecoder2.decodeKey(k)).map((k) => validName2.test(k) ? safeKey2(k) + ":r()" : "[" + JSON.stringify(k) + "]:r()").join(",") + "}") : new Function("r", "return {" + array.map((key) => validName2.test(key) ? safeKey2(key) + ":r()" : "[" + JSON.stringify(key) + "]:r()").join(",") + "}");
      if (this.compiledReader)
        compiledReader.next = this.compiledReader;
      compiledReader.propertyCount = length;
      this.compiledReader = compiledReader;
      return compiledReader(read2);
    }
    let object = {};
    if (currentDecoder2.keyMap)
      for (let i = 0; i < length; i++)
        object[safeKey2(currentDecoder2.decodeKey(this[i]))] = read2();
    else
      for (let i = 0; i < length; i++) {
        object[safeKey2(this[i])] = read2();
      }
    return object;
  }
  structure.slowReads = 0;
  return readObject;
}
function safeKey2(key) {
  return key === "__proto__" ? "__proto_" : key;
}
var readFixedString2 = readStringJS2;
function readStringJS2(length) {
  let result;
  if (length < 16) {
    if (result = shortStringInJS2(length))
      return result;
  }
  if (length > 64 && decoder2)
    return decoder2.decode(src2.subarray(position3, position3 += length));
  const end = position3 + length;
  const units = [];
  result = "";
  while (position3 < end) {
    const byte1 = src2[position3++];
    if ((byte1 & 128) === 0) {
      units.push(byte1);
    } else if ((byte1 & 224) === 192) {
      const byte2 = src2[position3++] & 63;
      units.push((byte1 & 31) << 6 | byte2);
    } else if ((byte1 & 240) === 224) {
      const byte2 = src2[position3++] & 63;
      const byte3 = src2[position3++] & 63;
      units.push((byte1 & 31) << 12 | byte2 << 6 | byte3);
    } else if ((byte1 & 248) === 240) {
      const byte2 = src2[position3++] & 63;
      const byte3 = src2[position3++] & 63;
      const byte4 = src2[position3++] & 63;
      let unit = (byte1 & 7) << 18 | byte2 << 12 | byte3 << 6 | byte4;
      if (unit > 65535) {
        unit -= 65536;
        units.push(unit >>> 10 & 1023 | 55296);
        unit = 56320 | unit & 1023;
      }
      units.push(unit);
    } else {
      units.push(byte1);
    }
    if (units.length >= 4096) {
      result += fromCharCode2.apply(String, units);
      units.length = 0;
    }
  }
  if (units.length > 0) {
    result += fromCharCode2.apply(String, units);
  }
  return result;
}
var fromCharCode2 = String.fromCharCode;
function longStringInJS2(length) {
  let start = position3;
  let bytes = new Array(length);
  for (let i = 0; i < length; i++) {
    const byte = src2[position3++];
    if ((byte & 128) > 0) {
      position3 = start;
      return;
    }
    bytes[i] = byte;
  }
  return fromCharCode2.apply(String, bytes);
}
function shortStringInJS2(length) {
  if (length < 4) {
    if (length < 2) {
      if (length === 0)
        return "";
      else {
        let a = src2[position3++];
        if ((a & 128) > 1) {
          position3 -= 1;
          return;
        }
        return fromCharCode2(a);
      }
    } else {
      let a = src2[position3++];
      let b = src2[position3++];
      if ((a & 128) > 0 || (b & 128) > 0) {
        position3 -= 2;
        return;
      }
      if (length < 3)
        return fromCharCode2(a, b);
      let c = src2[position3++];
      if ((c & 128) > 0) {
        position3 -= 3;
        return;
      }
      return fromCharCode2(a, b, c);
    }
  } else {
    let a = src2[position3++];
    let b = src2[position3++];
    let c = src2[position3++];
    let d = src2[position3++];
    if ((a & 128) > 0 || (b & 128) > 0 || (c & 128) > 0 || (d & 128) > 0) {
      position3 -= 4;
      return;
    }
    if (length < 6) {
      if (length === 4)
        return fromCharCode2(a, b, c, d);
      else {
        let e = src2[position3++];
        if ((e & 128) > 0) {
          position3 -= 5;
          return;
        }
        return fromCharCode2(a, b, c, d, e);
      }
    } else if (length < 8) {
      let e = src2[position3++];
      let f = src2[position3++];
      if ((e & 128) > 0 || (f & 128) > 0) {
        position3 -= 6;
        return;
      }
      if (length < 7)
        return fromCharCode2(a, b, c, d, e, f);
      let g = src2[position3++];
      if ((g & 128) > 0) {
        position3 -= 7;
        return;
      }
      return fromCharCode2(a, b, c, d, e, f, g);
    } else {
      let e = src2[position3++];
      let f = src2[position3++];
      let g = src2[position3++];
      let h = src2[position3++];
      if ((e & 128) > 0 || (f & 128) > 0 || (g & 128) > 0 || (h & 128) > 0) {
        position3 -= 8;
        return;
      }
      if (length < 10) {
        if (length === 8)
          return fromCharCode2(a, b, c, d, e, f, g, h);
        else {
          let i = src2[position3++];
          if ((i & 128) > 0) {
            position3 -= 9;
            return;
          }
          return fromCharCode2(a, b, c, d, e, f, g, h, i);
        }
      } else if (length < 12) {
        let i = src2[position3++];
        let j = src2[position3++];
        if ((i & 128) > 0 || (j & 128) > 0) {
          position3 -= 10;
          return;
        }
        if (length < 11)
          return fromCharCode2(a, b, c, d, e, f, g, h, i, j);
        let k = src2[position3++];
        if ((k & 128) > 0) {
          position3 -= 11;
          return;
        }
        return fromCharCode2(a, b, c, d, e, f, g, h, i, j, k);
      } else {
        let i = src2[position3++];
        let j = src2[position3++];
        let k = src2[position3++];
        let l = src2[position3++];
        if ((i & 128) > 0 || (j & 128) > 0 || (k & 128) > 0 || (l & 128) > 0) {
          position3 -= 12;
          return;
        }
        if (length < 14) {
          if (length === 12)
            return fromCharCode2(a, b, c, d, e, f, g, h, i, j, k, l);
          else {
            let m = src2[position3++];
            if ((m & 128) > 0) {
              position3 -= 13;
              return;
            }
            return fromCharCode2(a, b, c, d, e, f, g, h, i, j, k, l, m);
          }
        } else {
          let m = src2[position3++];
          let n = src2[position3++];
          if ((m & 128) > 0 || (n & 128) > 0) {
            position3 -= 14;
            return;
          }
          if (length < 15)
            return fromCharCode2(a, b, c, d, e, f, g, h, i, j, k, l, m, n);
          let o = src2[position3++];
          if ((o & 128) > 0) {
            position3 -= 15;
            return;
          }
          return fromCharCode2(a, b, c, d, e, f, g, h, i, j, k, l, m, n, o);
        }
      }
    }
  }
}
function readBin2(length) {
  return currentDecoder2.copyBuffers ? (
    // specifically use the copying slice (not the node one)
    Uint8Array.prototype.slice.call(src2, position3, position3 += length)
  ) : src2.subarray(position3, position3 += length);
}
var f32Array2 = new Float32Array(1);
var u8Array2 = new Uint8Array(f32Array2.buffer, 0, 4);
function getFloat162() {
  let byte0 = src2[position3++];
  let byte1 = src2[position3++];
  let exponent = (byte0 & 127) >> 2;
  if (exponent === 31) {
    if (byte1 || byte0 & 3)
      return NaN;
    return byte0 & 128 ? -Infinity : Infinity;
  }
  if (exponent === 0) {
    let abs = ((byte0 & 3) << 8 | byte1) / (1 << 24);
    return byte0 & 128 ? -abs : abs;
  }
  u8Array2[3] = byte0 & 128 | // sign bit
  (exponent >> 1) + 56;
  u8Array2[2] = (byte0 & 7) << 5 | // last exponent bit and first two mantissa bits
  byte1 >> 3;
  u8Array2[1] = byte1 << 5;
  u8Array2[0] = 0;
  return f32Array2[0];
}
var keyCache2 = new Array(4096);
var Tag2 = class {
  constructor(value, tag) {
    this.value = value;
    this.tag = tag;
  }
};
currentExtensions2[0] = (dateString) => {
  return new Date(dateString);
};
currentExtensions2[1] = (epochSec) => {
  return new Date(Math.round(epochSec * 1e3));
};
currentExtensions2[2] = (buffer) => {
  let value = BigInt(0);
  for (let i = 0, l = buffer.byteLength; i < l; i++) {
    value = BigInt(buffer[i]) + value << BigInt(8);
  }
  return value;
};
currentExtensions2[3] = (buffer) => {
  return BigInt(-1) - currentExtensions2[2](buffer);
};
currentExtensions2[4] = (fraction) => {
  return +(fraction[1] + "e" + fraction[0]);
};
currentExtensions2[5] = (fraction) => {
  return fraction[1] * Math.exp(fraction[0] * Math.log(2));
};
var recordDefinition2 = (id, structure) => {
  id = id - 57344;
  let existingStructure = currentStructures2[id];
  if (existingStructure && existingStructure.isShared) {
    (currentStructures2.restoreStructures || (currentStructures2.restoreStructures = []))[id] = existingStructure;
  }
  currentStructures2[id] = structure;
  structure.read = createStructureReader2(structure);
};
currentExtensions2[LEGACY_RECORD_INLINE_ID2] = (data) => {
  let length = data.length;
  let structure = data[1];
  recordDefinition2(data[0], structure);
  let object = {};
  for (let i = 2; i < length; i++) {
    let key = structure[i - 2];
    object[safeKey2(key)] = data[i];
  }
  return object;
};
currentExtensions2[14] = (value) => {
  if (bundledStrings3)
    return bundledStrings3[0].slice(bundledStrings3.position0, bundledStrings3.position0 += value);
  return new Tag2(value, 14);
};
currentExtensions2[15] = (value) => {
  if (bundledStrings3)
    return bundledStrings3[1].slice(bundledStrings3.position1, bundledStrings3.position1 += value);
  return new Tag2(value, 15);
};
var glbl2 = { Error, RegExp };
currentExtensions2[27] = (data) => {
  return (glbl2[data[0]] || Error)(data[1], data[2]);
};
var packedTable2 = (read3) => {
  if (src2[position3++] != 132)
    throw new Error("Packed values structure must be followed by a 4 element array");
  let newPackedValues = read3();
  packedValues2 = packedValues2 ? newPackedValues.concat(packedValues2.slice(newPackedValues.length)) : newPackedValues;
  packedValues2.prefixes = read3();
  packedValues2.suffixes = read3();
  return read3();
};
packedTable2.handlesRead = true;
currentExtensions2[51] = packedTable2;
currentExtensions2[PACKED_REFERENCE_TAG_ID2] = (data) => {
  if (!packedValues2) {
    if (currentDecoder2.getShared)
      loadShared2();
    else
      return new Tag2(data, PACKED_REFERENCE_TAG_ID2);
  }
  if (typeof data == "number")
    return packedValues2[16 + (data >= 0 ? 2 * data : -2 * data - 1)];
  throw new Error("No support for non-integer packed references yet");
};
currentExtensions2[28] = (read3) => {
  if (!referenceMap2) {
    referenceMap2 = /* @__PURE__ */ new Map();
    referenceMap2.id = 0;
  }
  let id = referenceMap2.id++;
  let token = src2[position3];
  let target3;
  if (token >> 5 == 4)
    target3 = [];
  else
    target3 = {};
  let refEntry = { target: target3 };
  referenceMap2.set(id, refEntry);
  let targetProperties = read3();
  if (refEntry.used)
    return Object.assign(target3, targetProperties);
  refEntry.target = targetProperties;
  return targetProperties;
};
currentExtensions2[28].handlesRead = true;
currentExtensions2[29] = (id) => {
  let refEntry = referenceMap2.get(id);
  refEntry.used = true;
  return refEntry.target;
};
currentExtensions2[258] = (array) => new Set(array);
(currentExtensions2[259] = (read3) => {
  if (currentDecoder2.mapsAsObjects) {
    currentDecoder2.mapsAsObjects = false;
    restoreMapsAsObject2 = true;
  }
  return read3();
}).handlesRead = true;
function combine2(a, b) {
  if (typeof a === "string")
    return a + b;
  if (a instanceof Array)
    return a.concat(b);
  return Object.assign({}, a, b);
}
function getPackedValues2() {
  if (!packedValues2) {
    if (currentDecoder2.getShared)
      loadShared2();
    else
      throw new Error("No packed values available");
  }
  return packedValues2;
}
var SHARED_DATA_TAG_ID2 = 1399353956;
currentExtensionRanges2.push((tag, input) => {
  if (tag >= 225 && tag <= 255)
    return combine2(getPackedValues2().prefixes[tag - 224], input);
  if (tag >= 28704 && tag <= 32767)
    return combine2(getPackedValues2().prefixes[tag - 28672], input);
  if (tag >= 1879052288 && tag <= 2147483647)
    return combine2(getPackedValues2().prefixes[tag - 1879048192], input);
  if (tag >= 216 && tag <= 223)
    return combine2(input, getPackedValues2().suffixes[tag - 216]);
  if (tag >= 27647 && tag <= 28671)
    return combine2(input, getPackedValues2().suffixes[tag - 27639]);
  if (tag >= 1811940352 && tag <= 1879048191)
    return combine2(input, getPackedValues2().suffixes[tag - 1811939328]);
  if (tag == SHARED_DATA_TAG_ID2) {
    return {
      packedValues: packedValues2,
      structures: currentStructures2.slice(0),
      version: input
    };
  }
  if (tag == 55799)
    return input;
});
var isLittleEndianMachine3 = new Uint8Array(new Uint16Array([1]).buffer)[0] == 1;
var typedArrays2 = [
  Uint8Array,
  Uint8ClampedArray,
  Uint16Array,
  Uint32Array,
  typeof BigUint64Array == "undefined" ? { name: "BigUint64Array" } : BigUint64Array,
  Int8Array,
  Int16Array,
  Int32Array,
  typeof BigInt64Array == "undefined" ? { name: "BigInt64Array" } : BigInt64Array,
  Float32Array,
  Float64Array
];
var typedArrayTags2 = [64, 68, 69, 70, 71, 72, 77, 78, 79, 85, 86];
for (let i = 0; i < typedArrays2.length; i++) {
  registerTypedArray2(typedArrays2[i], typedArrayTags2[i]);
}
function registerTypedArray2(TypedArray, tag) {
  let dvMethod = "get" + TypedArray.name.slice(0, -5);
  if (typeof TypedArray !== "function")
    TypedArray = null;
  let bytesPerElement = TypedArray.BYTES_PER_ELEMENT;
  for (let littleEndian = 0; littleEndian < 2; littleEndian++) {
    if (!littleEndian && bytesPerElement == 1)
      continue;
    let sizeShift = bytesPerElement == 2 ? 1 : bytesPerElement == 4 ? 2 : 3;
    currentExtensions2[littleEndian ? tag : tag - 4] = bytesPerElement == 1 || littleEndian == isLittleEndianMachine3 ? (buffer) => {
      if (!TypedArray)
        throw new Error("Could not find typed array for code " + tag);
      return new TypedArray(Uint8Array.prototype.slice.call(buffer, 0).buffer);
    } : (buffer) => {
      if (!TypedArray)
        throw new Error("Could not find typed array for code " + tag);
      let dv = new DataView(buffer.buffer, buffer.byteOffset, buffer.byteLength);
      let elements = buffer.length >> sizeShift;
      let ta = new TypedArray(elements);
      let method = dv[dvMethod];
      for (let i = 0; i < elements; i++) {
        ta[i] = method.call(dv, i << sizeShift, littleEndian);
      }
      return ta;
    };
  }
}
function readBundleExt2() {
  let length = readJustLength2();
  let bundlePosition = position3 + read2();
  for (let i = 2; i < length; i++) {
    let bundleLength = readJustLength2();
    position3 += bundleLength;
  }
  let dataPosition = position3;
  position3 = bundlePosition;
  bundledStrings3 = [readStringJS2(readJustLength2()), readStringJS2(readJustLength2())];
  bundledStrings3.position0 = 0;
  bundledStrings3.position1 = 0;
  bundledStrings3.postBundlePosition = position3;
  position3 = dataPosition;
  return read2();
}
function readJustLength2() {
  let token = src2[position3++] & 31;
  if (token > 23) {
    switch (token) {
      case 24:
        token = src2[position3++];
        break;
      case 25:
        token = dataView2.getUint16(position3);
        position3 += 2;
        break;
      case 26:
        token = dataView2.getUint32(position3);
        position3 += 4;
        break;
    }
  }
  return token;
}
function loadShared2() {
  if (currentDecoder2.getShared) {
    let sharedData = saveState2(() => {
      src2 = null;
      return currentDecoder2.getShared();
    }) || {};
    let updatedStructures = sharedData.structures || [];
    currentDecoder2.sharedVersion = sharedData.version;
    packedValues2 = currentDecoder2.sharedValues = sharedData.packedValues;
    if (currentStructures2 === true)
      currentDecoder2.structures = currentStructures2 = updatedStructures;
    else
      currentStructures2.splice.apply(currentStructures2, [0, updatedStructures.length].concat(updatedStructures));
  }
}
function saveState2(callback) {
  let savedSrcEnd = srcEnd2;
  let savedPosition = position3;
  let savedStringPosition = stringPosition2;
  let savedSrcStringStart = srcStringStart2;
  let savedSrcStringEnd = srcStringEnd2;
  let savedSrcString = srcString2;
  let savedStrings = strings2;
  let savedReferenceMap = referenceMap2;
  let savedBundledStrings = bundledStrings3;
  let savedSrc = new Uint8Array(src2.slice(0, srcEnd2));
  let savedStructures = currentStructures2;
  let savedDecoder = currentDecoder2;
  let savedSequentialMode = sequentialMode2;
  let value = callback();
  srcEnd2 = savedSrcEnd;
  position3 = savedPosition;
  stringPosition2 = savedStringPosition;
  srcStringStart2 = savedSrcStringStart;
  srcStringEnd2 = savedSrcStringEnd;
  srcString2 = savedSrcString;
  strings2 = savedStrings;
  referenceMap2 = savedReferenceMap;
  bundledStrings3 = savedBundledStrings;
  src2 = savedSrc;
  sequentialMode2 = savedSequentialMode;
  currentStructures2 = savedStructures;
  currentDecoder2 = savedDecoder;
  dataView2 = new DataView(src2.buffer, src2.byteOffset, src2.byteLength);
  return value;
}
function clearSource2() {
  src2 = null;
  referenceMap2 = null;
  currentStructures2 = null;
}
function addExtension3(extension) {
  currentExtensions2[extension.tag] = extension.decode;
}
var mult102 = new Array(147);
for (let i = 0; i < 256; i++) {
  mult102[i] = +("1e" + Math.floor(45.15 - i * 0.30103));
}
var defaultDecoder2 = new Decoder2({ useRecords: false });
var decode2 = defaultDecoder2.decode;
var decodeMultiple2 = defaultDecoder2.decodeMultiple;
var FLOAT32_OPTIONS2 = {
  NEVER: 0,
  ALWAYS: 1,
  DECIMAL_ROUND: 3,
  DECIMAL_FIT: 4
};

// node_modules/@progrium/duplex/esm/vnd/cbor-x-1.4.1/encode.js
var textEncoder2;
try {
  textEncoder2 = new TextEncoder();
} catch (error) {
}
var extensions2;
var extensionClasses2;
var Buffer3 = globalThis.Buffer;
var hasNodeBuffer2 = typeof Buffer3 !== "undefined";
var ByteArrayAllocate2 = hasNodeBuffer2 ? Buffer3.allocUnsafeSlow : Uint8Array;
var ByteArray2 = hasNodeBuffer2 ? Buffer3 : Uint8Array;
var MAX_STRUCTURES2 = 256;
var MAX_BUFFER_SIZE2 = hasNodeBuffer2 ? 4294967296 : 2144337920;
var throwOnIterable2;
var target2;
var targetView2;
var position4 = 0;
var safeEnd2;
var bundledStrings4 = null;
var MAX_BUNDLE_SIZE2 = 61440;
var hasNonLatin2 = /[\u0080-\uFFFF]/;
var RECORD_SYMBOL2 = Symbol("record-id");
var Encoder2 = class extends Decoder2 {
  constructor(options) {
    super(options);
    this.offset = 0;
    let typeBuffer;
    let start;
    let sharedStructures;
    let hasSharedUpdate;
    let structures;
    let referenceMap3;
    options = options || {};
    let encodeUtf8 = ByteArray2.prototype.utf8Write ? function(string, position5, maxBytes) {
      return target2.utf8Write(string, position5, maxBytes);
    } : textEncoder2 && textEncoder2.encodeInto ? function(string, position5) {
      return textEncoder2.encodeInto(string, target2.subarray(position5)).written;
    } : false;
    let encoder = this;
    let hasSharedStructures = options.structures || options.saveStructures;
    let maxSharedStructures = options.maxSharedStructures;
    if (maxSharedStructures == null)
      maxSharedStructures = hasSharedStructures ? 128 : 0;
    if (maxSharedStructures > 8190)
      throw new Error("Maximum maxSharedStructure is 8190");
    let isSequential = options.sequential;
    if (isSequential) {
      maxSharedStructures = 0;
    }
    if (!this.structures)
      this.structures = [];
    if (this.saveStructures)
      this.saveShared = this.saveStructures;
    let samplingPackedValues, packedObjectMap2, sharedValues = options.sharedValues;
    let sharedPackedObjectMap2;
    if (sharedValues) {
      sharedPackedObjectMap2 = /* @__PURE__ */ Object.create(null);
      for (let i = 0, l = sharedValues.length; i < l; i++) {
        sharedPackedObjectMap2[sharedValues[i]] = i;
      }
    }
    let recordIdsToRemove = [];
    let transitionsCount = 0;
    let serializationsSinceTransitionRebuild = 0;
    this.mapEncode = function(value, encodeOptions) {
      if (this._keyMap && !this._mapped) {
        switch (value.constructor.name) {
          case "Array":
            value = value.map((r) => this.encodeKeys(r));
            break;
        }
      }
      return this.encode(value, encodeOptions);
    };
    this.encode = function(value, encodeOptions) {
      if (!target2) {
        target2 = new ByteArrayAllocate2(8192);
        targetView2 = new DataView(target2.buffer, 0, 8192);
        position4 = 0;
      }
      safeEnd2 = target2.length - 10;
      if (safeEnd2 - position4 < 2048) {
        target2 = new ByteArrayAllocate2(target2.length);
        targetView2 = new DataView(target2.buffer, 0, target2.length);
        safeEnd2 = target2.length - 10;
        position4 = 0;
      } else if (encodeOptions === REUSE_BUFFER_MODE2)
        position4 = position4 + 7 & 2147483640;
      start = position4;
      if (encoder.useSelfDescribedHeader) {
        targetView2.setUint32(position4, 3654940416);
        position4 += 3;
      }
      referenceMap3 = encoder.structuredClone ? /* @__PURE__ */ new Map() : null;
      if (encoder.bundleStrings && typeof value !== "string") {
        bundledStrings4 = [];
        bundledStrings4.size = Infinity;
      } else
        bundledStrings4 = null;
      sharedStructures = encoder.structures;
      if (sharedStructures) {
        if (sharedStructures.uninitialized) {
          let sharedData = encoder.getShared() || {};
          encoder.structures = sharedStructures = sharedData.structures || [];
          encoder.sharedVersion = sharedData.version;
          let sharedValues2 = encoder.sharedValues = sharedData.packedValues;
          if (sharedValues2) {
            sharedPackedObjectMap2 = {};
            for (let i = 0, l = sharedValues2.length; i < l; i++)
              sharedPackedObjectMap2[sharedValues2[i]] = i;
          }
        }
        let sharedStructuresLength = sharedStructures.length;
        if (sharedStructuresLength > maxSharedStructures && !isSequential)
          sharedStructuresLength = maxSharedStructures;
        if (!sharedStructures.transitions) {
          sharedStructures.transitions = /* @__PURE__ */ Object.create(null);
          for (let i = 0; i < sharedStructuresLength; i++) {
            let keys = sharedStructures[i];
            if (!keys)
              continue;
            let nextTransition, transition = sharedStructures.transitions;
            for (let j = 0, l = keys.length; j < l; j++) {
              if (transition[RECORD_SYMBOL2] === void 0)
                transition[RECORD_SYMBOL2] = i;
              let key = keys[j];
              nextTransition = transition[key];
              if (!nextTransition) {
                nextTransition = transition[key] = /* @__PURE__ */ Object.create(null);
              }
              transition = nextTransition;
            }
            transition[RECORD_SYMBOL2] = i | 1048576;
          }
        }
        if (!isSequential)
          sharedStructures.nextId = sharedStructuresLength;
      }
      if (hasSharedUpdate)
        hasSharedUpdate = false;
      structures = sharedStructures || [];
      packedObjectMap2 = sharedPackedObjectMap2;
      if (options.pack) {
        let packedValues3 = /* @__PURE__ */ new Map();
        packedValues3.values = [];
        packedValues3.encoder = encoder;
        packedValues3.maxValues = options.maxPrivatePackedValues || (sharedPackedObjectMap2 ? 16 : Infinity);
        packedValues3.objectMap = sharedPackedObjectMap2 || false;
        packedValues3.samplingPackedValues = samplingPackedValues;
        findRepetitiveStrings2(value, packedValues3);
        if (packedValues3.values.length > 0) {
          target2[position4++] = 216;
          target2[position4++] = 51;
          writeArrayHeader2(4);
          let valuesArray = packedValues3.values;
          encode3(valuesArray);
          writeArrayHeader2(0);
          writeArrayHeader2(0);
          packedObjectMap2 = Object.create(sharedPackedObjectMap2 || null);
          for (let i = 0, l = valuesArray.length; i < l; i++) {
            packedObjectMap2[valuesArray[i]] = i;
          }
        }
      }
      throwOnIterable2 = encodeOptions & THROW_ON_ITERABLE2;
      try {
        if (throwOnIterable2)
          return;
        encode3(value);
        if (bundledStrings4) {
          writeBundles2(start, encode3);
        }
        encoder.offset = position4;
        if (referenceMap3 && referenceMap3.idsToInsert) {
          position4 += referenceMap3.idsToInsert.length * 2;
          if (position4 > safeEnd2)
            makeRoom(position4);
          encoder.offset = position4;
          let serialized = insertIds2(target2.subarray(start, position4), referenceMap3.idsToInsert);
          referenceMap3 = null;
          return serialized;
        }
        if (encodeOptions & REUSE_BUFFER_MODE2) {
          target2.start = start;
          target2.end = position4;
          return target2;
        }
        return target2.subarray(start, position4);
      } finally {
        if (sharedStructures) {
          if (serializationsSinceTransitionRebuild < 10)
            serializationsSinceTransitionRebuild++;
          if (sharedStructures.length > maxSharedStructures)
            sharedStructures.length = maxSharedStructures;
          if (transitionsCount > 1e4) {
            sharedStructures.transitions = null;
            serializationsSinceTransitionRebuild = 0;
            transitionsCount = 0;
            if (recordIdsToRemove.length > 0)
              recordIdsToRemove = [];
          } else if (recordIdsToRemove.length > 0 && !isSequential) {
            for (let i = 0, l = recordIdsToRemove.length; i < l; i++) {
              recordIdsToRemove[i][RECORD_SYMBOL2] = void 0;
            }
            recordIdsToRemove = [];
          }
        }
        if (hasSharedUpdate && encoder.saveShared) {
          if (encoder.structures.length > maxSharedStructures) {
            encoder.structures = encoder.structures.slice(0, maxSharedStructures);
          }
          let returnBuffer = target2.subarray(start, position4);
          if (encoder.updateSharedData() === false)
            return encoder.encode(value);
          return returnBuffer;
        }
        if (encodeOptions & RESET_BUFFER_MODE2)
          position4 = start;
      }
    };
    this.findCommonStringsToPack = () => {
      samplingPackedValues = /* @__PURE__ */ new Map();
      if (!sharedPackedObjectMap2)
        sharedPackedObjectMap2 = /* @__PURE__ */ Object.create(null);
      return (options2) => {
        let threshold = options2 && options2.threshold || 4;
        let position5 = this.pack ? options2.maxPrivatePackedValues || 16 : 0;
        if (!sharedValues)
          sharedValues = this.sharedValues = [];
        for (let [key, status] of samplingPackedValues) {
          if (status.count > threshold) {
            sharedPackedObjectMap2[key] = position5++;
            sharedValues.push(key);
            hasSharedUpdate = true;
          }
        }
        while (this.saveShared && this.updateSharedData() === false) {
        }
        samplingPackedValues = null;
      };
    };
    const encode3 = (value) => {
      if (position4 > safeEnd2)
        target2 = makeRoom(position4);
      var type = typeof value;
      var length;
      if (type === "string") {
        if (packedObjectMap2) {
          let packedPosition = packedObjectMap2[value];
          if (packedPosition >= 0) {
            if (packedPosition < 16)
              target2[position4++] = packedPosition + 224;
            else {
              target2[position4++] = 198;
              if (packedPosition & 1)
                encode3(15 - packedPosition >> 1);
              else
                encode3(packedPosition - 16 >> 1);
            }
            return;
          } else if (samplingPackedValues && !options.pack) {
            let status = samplingPackedValues.get(value);
            if (status)
              status.count++;
            else
              samplingPackedValues.set(value, {
                count: 1
              });
          }
        }
        let strLength = value.length;
        if (bundledStrings4 && strLength >= 4 && strLength < 1024) {
          if ((bundledStrings4.size += strLength) > MAX_BUNDLE_SIZE2) {
            let extStart;
            let maxBytes2 = (bundledStrings4[0] ? bundledStrings4[0].length * 3 + bundledStrings4[1].length : 0) + 10;
            if (position4 + maxBytes2 > safeEnd2)
              target2 = makeRoom(position4 + maxBytes2);
            target2[position4++] = 217;
            target2[position4++] = 223;
            target2[position4++] = 249;
            target2[position4++] = bundledStrings4.position ? 132 : 130;
            target2[position4++] = 26;
            extStart = position4 - start;
            position4 += 4;
            if (bundledStrings4.position) {
              writeBundles2(start, encode3);
            }
            bundledStrings4 = ["", ""];
            bundledStrings4.size = 0;
            bundledStrings4.position = extStart;
          }
          let twoByte = hasNonLatin2.test(value);
          bundledStrings4[twoByte ? 0 : 1] += value;
          target2[position4++] = twoByte ? 206 : 207;
          encode3(strLength);
          return;
        }
        let headerSize;
        if (strLength < 32) {
          headerSize = 1;
        } else if (strLength < 256) {
          headerSize = 2;
        } else if (strLength < 65536) {
          headerSize = 3;
        } else {
          headerSize = 5;
        }
        let maxBytes = strLength * 3;
        if (position4 + maxBytes > safeEnd2)
          target2 = makeRoom(position4 + maxBytes);
        if (strLength < 64 || !encodeUtf8) {
          let i, c1, c2, strPosition = position4 + headerSize;
          for (i = 0; i < strLength; i++) {
            c1 = value.charCodeAt(i);
            if (c1 < 128) {
              target2[strPosition++] = c1;
            } else if (c1 < 2048) {
              target2[strPosition++] = c1 >> 6 | 192;
              target2[strPosition++] = c1 & 63 | 128;
            } else if ((c1 & 64512) === 55296 && ((c2 = value.charCodeAt(i + 1)) & 64512) === 56320) {
              c1 = 65536 + ((c1 & 1023) << 10) + (c2 & 1023);
              i++;
              target2[strPosition++] = c1 >> 18 | 240;
              target2[strPosition++] = c1 >> 12 & 63 | 128;
              target2[strPosition++] = c1 >> 6 & 63 | 128;
              target2[strPosition++] = c1 & 63 | 128;
            } else {
              target2[strPosition++] = c1 >> 12 | 224;
              target2[strPosition++] = c1 >> 6 & 63 | 128;
              target2[strPosition++] = c1 & 63 | 128;
            }
          }
          length = strPosition - position4 - headerSize;
        } else {
          length = encodeUtf8(value, position4 + headerSize, maxBytes);
        }
        if (length < 24) {
          target2[position4++] = 96 | length;
        } else if (length < 256) {
          if (headerSize < 2) {
            target2.copyWithin(position4 + 2, position4 + 1, position4 + 1 + length);
          }
          target2[position4++] = 120;
          target2[position4++] = length;
        } else if (length < 65536) {
          if (headerSize < 3) {
            target2.copyWithin(position4 + 3, position4 + 2, position4 + 2 + length);
          }
          target2[position4++] = 121;
          target2[position4++] = length >> 8;
          target2[position4++] = length & 255;
        } else {
          if (headerSize < 5) {
            target2.copyWithin(position4 + 5, position4 + 3, position4 + 3 + length);
          }
          target2[position4++] = 122;
          targetView2.setUint32(position4, length);
          position4 += 4;
        }
        position4 += length;
      } else if (type === "number") {
        if (!this.alwaysUseFloat && value >>> 0 === value) {
          if (value < 24) {
            target2[position4++] = value;
          } else if (value < 256) {
            target2[position4++] = 24;
            target2[position4++] = value;
          } else if (value < 65536) {
            target2[position4++] = 25;
            target2[position4++] = value >> 8;
            target2[position4++] = value & 255;
          } else {
            target2[position4++] = 26;
            targetView2.setUint32(position4, value);
            position4 += 4;
          }
        } else if (!this.alwaysUseFloat && value >> 0 === value) {
          if (value >= -24) {
            target2[position4++] = 31 - value;
          } else if (value >= -256) {
            target2[position4++] = 56;
            target2[position4++] = ~value;
          } else if (value >= -65536) {
            target2[position4++] = 57;
            targetView2.setUint16(position4, ~value);
            position4 += 2;
          } else {
            target2[position4++] = 58;
            targetView2.setUint32(position4, ~value);
            position4 += 4;
          }
        } else {
          let useFloat32;
          if ((useFloat32 = this.useFloat32) > 0 && value < 4294967296 && value >= -2147483648) {
            target2[position4++] = 250;
            targetView2.setFloat32(position4, value);
            let xShifted;
            if (useFloat32 < 4 || // this checks for rounding of numbers that were encoded in 32-bit float to nearest significant decimal digit that could be preserved
            (xShifted = value * mult102[(target2[position4] & 127) << 1 | target2[position4 + 1] >> 7]) >> 0 === xShifted) {
              position4 += 4;
              return;
            } else
              position4--;
          }
          target2[position4++] = 251;
          targetView2.setFloat64(position4, value);
          position4 += 8;
        }
      } else if (type === "object") {
        if (!value)
          target2[position4++] = 246;
        else {
          if (referenceMap3) {
            let referee = referenceMap3.get(value);
            if (referee) {
              target2[position4++] = 216;
              target2[position4++] = 29;
              target2[position4++] = 25;
              if (!referee.references) {
                let idsToInsert = referenceMap3.idsToInsert || (referenceMap3.idsToInsert = []);
                referee.references = [];
                idsToInsert.push(referee);
              }
              referee.references.push(position4 - start);
              position4 += 2;
              return;
            } else
              referenceMap3.set(value, { offset: position4 - start });
          }
          let constructor = value.constructor;
          if (constructor === Object) {
            writeObject(value, true);
          } else if (constructor === Array) {
            length = value.length;
            if (length < 24) {
              target2[position4++] = 128 | length;
            } else {
              writeArrayHeader2(length);
            }
            for (let i = 0; i < length; i++) {
              encode3(value[i]);
            }
          } else if (constructor === Map) {
            if (this.mapsAsObjects ? this.useTag259ForMaps !== false : this.useTag259ForMaps) {
              target2[position4++] = 217;
              target2[position4++] = 1;
              target2[position4++] = 3;
            }
            length = value.size;
            if (length < 24) {
              target2[position4++] = 160 | length;
            } else if (length < 256) {
              target2[position4++] = 184;
              target2[position4++] = length;
            } else if (length < 65536) {
              target2[position4++] = 185;
              target2[position4++] = length >> 8;
              target2[position4++] = length & 255;
            } else {
              target2[position4++] = 186;
              targetView2.setUint32(position4, length);
              position4 += 4;
            }
            if (encoder.keyMap) {
              for (let [key, entryValue] of value) {
                encode3(encoder.encodeKey(key));
                encode3(entryValue);
              }
            } else {
              for (let [key, entryValue] of value) {
                encode3(key);
                encode3(entryValue);
              }
            }
          } else {
            for (let i = 0, l = extensions2.length; i < l; i++) {
              let extensionClass = extensionClasses2[i];
              if (value instanceof extensionClass) {
                let extension = extensions2[i];
                let tag = extension.tag;
                if (tag == void 0)
                  tag = extension.getTag && extension.getTag.call(this, value);
                if (tag < 24) {
                  target2[position4++] = 192 | tag;
                } else if (tag < 256) {
                  target2[position4++] = 216;
                  target2[position4++] = tag;
                } else if (tag < 65536) {
                  target2[position4++] = 217;
                  target2[position4++] = tag >> 8;
                  target2[position4++] = tag & 255;
                } else if (tag > -1) {
                  target2[position4++] = 218;
                  targetView2.setUint32(position4, tag);
                  position4 += 4;
                }
                extension.encode.call(this, value, encode3, makeRoom);
                return;
              }
            }
            if (value[Symbol.iterator]) {
              if (throwOnIterable2) {
                let error = new Error("Iterable should be serialized as iterator");
                error.iteratorNotHandled = true;
                throw error;
              }
              target2[position4++] = 159;
              for (let entry of value) {
                encode3(entry);
              }
              target2[position4++] = 255;
              return;
            }
            if (value[Symbol.asyncIterator] || isBlob2(value)) {
              let error = new Error("Iterable/blob should be serialized as iterator");
              error.iteratorNotHandled = true;
              throw error;
            }
            writeObject(value, !value.hasOwnProperty);
          }
        }
      } else if (type === "boolean") {
        target2[position4++] = value ? 245 : 244;
      } else if (type === "bigint") {
        if (value < BigInt(1) << BigInt(64) && value >= 0) {
          target2[position4++] = 27;
          targetView2.setBigUint64(position4, value);
        } else if (value > -(BigInt(1) << BigInt(64)) && value < 0) {
          target2[position4++] = 59;
          targetView2.setBigUint64(position4, -value - BigInt(1));
        } else {
          if (this.largeBigIntToFloat) {
            target2[position4++] = 251;
            targetView2.setFloat64(position4, Number(value));
          } else {
            throw new RangeError(value + " was too large to fit in CBOR 64-bit integer format, set largeBigIntToFloat to convert to float-64");
          }
        }
        position4 += 8;
      } else if (type === "undefined") {
        target2[position4++] = 247;
      } else {
        throw new Error("Unknown type: " + type);
      }
    };
    const writeObject = this.useRecords === false ? this.variableMapSize ? (object) => {
      let keys = Object.keys(object);
      let vals = Object.values(object);
      let length = keys.length;
      if (length < 24) {
        target2[position4++] = 160 | length;
      } else if (length < 256) {
        target2[position4++] = 184;
        target2[position4++] = length;
      } else if (length < 65536) {
        target2[position4++] = 185;
        target2[position4++] = length >> 8;
        target2[position4++] = length & 255;
      } else {
        target2[position4++] = 186;
        targetView2.setUint32(position4, length);
        position4 += 4;
      }
      let key;
      if (encoder.keyMap) {
        for (let i = 0; i < length; i++) {
          encode3(encodeKey(keys[i]));
          encode3(vals[i]);
        }
      } else {
        for (let i = 0; i < length; i++) {
          encode3(keys[i]);
          encode3(vals[i]);
        }
      }
    } : (object, safePrototype) => {
      target2[position4++] = 185;
      let objectOffset = position4 - start;
      position4 += 2;
      let size = 0;
      if (encoder.keyMap) {
        for (let key in object)
          if (safePrototype || object.hasOwnProperty(key)) {
            encode3(encoder.encodeKey(key));
            encode3(object[key]);
            size++;
          }
      } else {
        for (let key in object)
          if (safePrototype || object.hasOwnProperty(key)) {
            encode3(key);
            encode3(object[key]);
            size++;
          }
      }
      target2[objectOffset++ + start] = size >> 8;
      target2[objectOffset + start] = size & 255;
    } : (object, safePrototype) => {
      let nextTransition, transition = structures.transitions || (structures.transitions = /* @__PURE__ */ Object.create(null));
      let newTransitions = 0;
      let length = 0;
      let parentRecordId;
      let keys;
      if (this.keyMap) {
        keys = Object.keys(object).map((k) => this.encodeKey(k));
        length = keys.length;
        for (let i = 0; i < length; i++) {
          let key = keys[i];
          nextTransition = transition[key];
          if (!nextTransition) {
            nextTransition = transition[key] = /* @__PURE__ */ Object.create(null);
            newTransitions++;
          }
          transition = nextTransition;
        }
      } else {
        for (let key in object)
          if (safePrototype || object.hasOwnProperty(key)) {
            nextTransition = transition[key];
            if (!nextTransition) {
              if (transition[RECORD_SYMBOL2] & 1048576) {
                parentRecordId = transition[RECORD_SYMBOL2] & 65535;
              }
              nextTransition = transition[key] = /* @__PURE__ */ Object.create(null);
              newTransitions++;
            }
            transition = nextTransition;
            length++;
          }
      }
      let recordId = transition[RECORD_SYMBOL2];
      if (recordId !== void 0) {
        recordId &= 65535;
        target2[position4++] = 217;
        target2[position4++] = recordId >> 8 | 224;
        target2[position4++] = recordId & 255;
      } else {
        if (!keys)
          keys = transition.__keys__ || (transition.__keys__ = Object.keys(object));
        if (parentRecordId === void 0) {
          recordId = structures.nextId++;
          if (!recordId) {
            recordId = 0;
            structures.nextId = 1;
          }
          if (recordId >= MAX_STRUCTURES2) {
            structures.nextId = (recordId = maxSharedStructures) + 1;
          }
        } else {
          recordId = parentRecordId;
        }
        structures[recordId] = keys;
        if (recordId < maxSharedStructures) {
          target2[position4++] = 217;
          target2[position4++] = recordId >> 8 | 224;
          target2[position4++] = recordId & 255;
          transition = structures.transitions;
          for (let i = 0; i < length; i++) {
            if (transition[RECORD_SYMBOL2] === void 0 || transition[RECORD_SYMBOL2] & 1048576)
              transition[RECORD_SYMBOL2] = recordId;
            transition = transition[keys[i]];
          }
          transition[RECORD_SYMBOL2] = recordId | 1048576;
          hasSharedUpdate = true;
        } else {
          transition[RECORD_SYMBOL2] = recordId;
          targetView2.setUint32(position4, 3655335680);
          position4 += 3;
          if (newTransitions)
            transitionsCount += serializationsSinceTransitionRebuild * newTransitions;
          if (recordIdsToRemove.length >= MAX_STRUCTURES2 - maxSharedStructures)
            recordIdsToRemove.shift()[RECORD_SYMBOL2] = void 0;
          recordIdsToRemove.push(transition);
          writeArrayHeader2(length + 2);
          encode3(57344 + recordId);
          encode3(keys);
          if (safePrototype === null)
            return;
          for (let key in object)
            if (safePrototype || object.hasOwnProperty(key))
              encode3(object[key]);
          return;
        }
      }
      if (length < 24) {
        target2[position4++] = 128 | length;
      } else {
        writeArrayHeader2(length);
      }
      if (safePrototype === null)
        return;
      for (let key in object)
        if (safePrototype || object.hasOwnProperty(key))
          encode3(object[key]);
    };
    const makeRoom = (end) => {
      let newSize;
      if (end > 16777216) {
        if (end - start > MAX_BUFFER_SIZE2)
          throw new Error("Encoded buffer would be larger than maximum buffer size");
        newSize = Math.min(MAX_BUFFER_SIZE2, Math.round(Math.max((end - start) * (end > 67108864 ? 1.25 : 2), 4194304) / 4096) * 4096);
      } else
        newSize = (Math.max(end - start << 2, target2.length - 1) >> 12) + 1 << 12;
      let newBuffer = new ByteArrayAllocate2(newSize);
      targetView2 = new DataView(newBuffer.buffer, 0, newSize);
      if (target2.copy)
        target2.copy(newBuffer, 0, start, end);
      else
        newBuffer.set(target2.slice(start, end));
      position4 -= start;
      start = 0;
      safeEnd2 = newBuffer.length - 10;
      return target2 = newBuffer;
    };
    let chunkThreshold = 100;
    let continuedChunkThreshold = 1e3;
    this.encodeAsIterable = function(value, options2) {
      return startEncoding(value, options2, encodeObjectAsIterable);
    };
    this.encodeAsAsyncIterable = function(value, options2) {
      return startEncoding(value, options2, encodeObjectAsAsyncIterable);
    };
    function* encodeObjectAsIterable(object, iterateProperties, finalIterable) {
      let constructor = object.constructor;
      if (constructor === Object) {
        let useRecords = encoder.useRecords !== false;
        if (useRecords)
          writeObject(object, null);
        else
          writeEntityLength2(Object.keys(object).length, 160);
        for (let key in object) {
          let value = object[key];
          if (!useRecords)
            encode3(key);
          if (value && typeof value === "object") {
            if (iterateProperties[key])
              yield* encodeObjectAsIterable(value, iterateProperties[key]);
            else
              yield* tryEncode(value, iterateProperties, key);
          } else
            encode3(value);
        }
      } else if (constructor === Array) {
        let length = object.length;
        writeArrayHeader2(length);
        for (let i = 0; i < length; i++) {
          let value = object[i];
          if (value && (typeof value === "object" || position4 - start > chunkThreshold)) {
            if (iterateProperties.element)
              yield* encodeObjectAsIterable(value, iterateProperties.element);
            else
              yield* tryEncode(value, iterateProperties, "element");
          } else
            encode3(value);
        }
      } else if (object[Symbol.iterator]) {
        target2[position4++] = 159;
        for (let value of object) {
          if (value && (typeof value === "object" || position4 - start > chunkThreshold)) {
            if (iterateProperties.element)
              yield* encodeObjectAsIterable(value, iterateProperties.element);
            else
              yield* tryEncode(value, iterateProperties, "element");
          } else
            encode3(value);
        }
        target2[position4++] = 255;
      } else if (isBlob2(object)) {
        writeEntityLength2(object.size, 64);
        yield target2.subarray(start, position4);
        yield object;
        restartEncoding();
      } else if (object[Symbol.asyncIterator]) {
        target2[position4++] = 159;
        yield target2.subarray(start, position4);
        yield object;
        restartEncoding();
        target2[position4++] = 255;
      } else {
        encode3(object);
      }
      if (finalIterable && position4 > start)
        yield target2.subarray(start, position4);
      else if (position4 - start > chunkThreshold) {
        yield target2.subarray(start, position4);
        restartEncoding();
      }
    }
    function* tryEncode(value, iterateProperties, key) {
      let restart = position4 - start;
      try {
        encode3(value);
        if (position4 - start > chunkThreshold) {
          yield target2.subarray(start, position4);
          restartEncoding();
        }
      } catch (error) {
        if (error.iteratorNotHandled) {
          iterateProperties[key] = {};
          position4 = start + restart;
          yield* encodeObjectAsIterable.call(this, value, iterateProperties[key]);
        } else
          throw error;
      }
    }
    function restartEncoding() {
      chunkThreshold = continuedChunkThreshold;
      encoder.encode(null, THROW_ON_ITERABLE2);
    }
    function startEncoding(value, options2, encodeIterable) {
      if (options2 && options2.chunkThreshold)
        chunkThreshold = continuedChunkThreshold = options2.chunkThreshold;
      else
        chunkThreshold = 100;
      if (value && typeof value === "object") {
        encoder.encode(null, THROW_ON_ITERABLE2);
        return encodeIterable(value, encoder.iterateProperties || (encoder.iterateProperties = {}), true);
      }
      return [encoder.encode(value)];
    }
    async function* encodeObjectAsAsyncIterable(value, iterateProperties) {
      for (let encodedValue of encodeObjectAsIterable(value, iterateProperties, true)) {
        let constructor = encodedValue.constructor;
        if (constructor === ByteArray2 || constructor === Uint8Array)
          yield encodedValue;
        else if (isBlob2(encodedValue)) {
          let reader = encodedValue.stream().getReader();
          let next;
          while (!(next = await reader.read()).done) {
            yield next.value;
          }
        } else if (encodedValue[Symbol.asyncIterator]) {
          for await (let asyncValue of encodedValue) {
            restartEncoding();
            if (asyncValue)
              yield* encodeObjectAsAsyncIterable(asyncValue, iterateProperties.async || (iterateProperties.async = {}));
            else
              yield encoder.encode(asyncValue);
          }
        } else {
          yield encodedValue;
        }
      }
    }
  }
  useBuffer(buffer) {
    target2 = buffer;
    targetView2 = new DataView(target2.buffer, target2.byteOffset, target2.byteLength);
    position4 = 0;
  }
  clearSharedData() {
    if (this.structures)
      this.structures = [];
    if (this.sharedValues)
      this.sharedValues = void 0;
  }
  updateSharedData() {
    let lastVersion = this.sharedVersion || 0;
    this.sharedVersion = lastVersion + 1;
    let structuresCopy = this.structures.slice(0);
    let sharedData = new SharedData2(structuresCopy, this.sharedValues, this.sharedVersion);
    let saveResults = this.saveShared(sharedData, (existingShared) => (existingShared && existingShared.version || 0) == lastVersion);
    if (saveResults === false) {
      sharedData = this.getShared() || {};
      this.structures = sharedData.structures || [];
      this.sharedValues = sharedData.packedValues;
      this.sharedVersion = sharedData.version;
      this.structures.nextId = this.structures.length;
    } else {
      structuresCopy.forEach((structure, i) => this.structures[i] = structure);
    }
    return saveResults;
  }
};
function writeEntityLength2(length, majorValue) {
  if (length < 24)
    target2[position4++] = majorValue | length;
  else if (length < 256) {
    target2[position4++] = majorValue | 24;
    target2[position4++] = length;
  } else if (length < 65536) {
    target2[position4++] = majorValue | 25;
    target2[position4++] = length >> 8;
    target2[position4++] = length & 255;
  } else {
    target2[position4++] = majorValue | 26;
    targetView2.setUint32(position4, length);
    position4 += 4;
  }
}
var SharedData2 = class {
  constructor(structures, values, version) {
    this.structures = structures;
    this.packedValues = values;
    this.version = version;
  }
};
function writeArrayHeader2(length) {
  if (length < 24)
    target2[position4++] = 128 | length;
  else if (length < 256) {
    target2[position4++] = 152;
    target2[position4++] = length;
  } else if (length < 65536) {
    target2[position4++] = 153;
    target2[position4++] = length >> 8;
    target2[position4++] = length & 255;
  } else {
    target2[position4++] = 154;
    targetView2.setUint32(position4, length);
    position4 += 4;
  }
}
var BlobConstructor2 = typeof Blob === "undefined" ? function() {
} : Blob;
function isBlob2(object) {
  if (object instanceof BlobConstructor2)
    return true;
  let tag = object[Symbol.toStringTag];
  return tag === "Blob" || tag === "File";
}
function findRepetitiveStrings2(value, packedValues3) {
  switch (typeof value) {
    case "string":
      if (value.length > 3) {
        if (packedValues3.objectMap[value] > -1 || packedValues3.values.length >= packedValues3.maxValues)
          return;
        let packedStatus = packedValues3.get(value);
        if (packedStatus) {
          if (++packedStatus.count == 2) {
            packedValues3.values.push(value);
          }
        } else {
          packedValues3.set(value, {
            count: 1
          });
          if (packedValues3.samplingPackedValues) {
            let status = packedValues3.samplingPackedValues.get(value);
            if (status)
              status.count++;
            else
              packedValues3.samplingPackedValues.set(value, {
                count: 1
              });
          }
        }
      }
      break;
    case "object":
      if (value) {
        if (value instanceof Array) {
          for (let i = 0, l = value.length; i < l; i++) {
            findRepetitiveStrings2(value[i], packedValues3);
          }
        } else {
          let includeKeys = !packedValues3.encoder.useRecords;
          for (var key in value) {
            if (value.hasOwnProperty(key)) {
              if (includeKeys)
                findRepetitiveStrings2(key, packedValues3);
              findRepetitiveStrings2(value[key], packedValues3);
            }
          }
        }
      }
      break;
    case "function":
      console.log(value);
  }
}
var isLittleEndianMachine4 = new Uint8Array(new Uint16Array([1]).buffer)[0] == 1;
extensionClasses2 = [
  Date,
  Set,
  Error,
  RegExp,
  Tag2,
  ArrayBuffer,
  Uint8Array,
  Uint8ClampedArray,
  Uint16Array,
  Uint32Array,
  typeof BigUint64Array == "undefined" ? function() {
  } : BigUint64Array,
  Int8Array,
  Int16Array,
  Int32Array,
  typeof BigInt64Array == "undefined" ? function() {
  } : BigInt64Array,
  Float32Array,
  Float64Array,
  SharedData2
];
extensions2 = [
  {
    tag: 1,
    encode(date, encode3) {
      let seconds = date.getTime() / 1e3;
      if ((this.useTimestamp32 || date.getMilliseconds() === 0) && seconds >= 0 && seconds < 4294967296) {
        target2[position4++] = 26;
        targetView2.setUint32(position4, seconds);
        position4 += 4;
      } else {
        target2[position4++] = 251;
        targetView2.setFloat64(position4, seconds);
        position4 += 8;
      }
    }
  },
  {
    tag: 258,
    encode(set, encode3) {
      let array = Array.from(set);
      encode3(array);
    }
  },
  {
    tag: 27,
    encode(error, encode3) {
      encode3([error.name, error.message]);
    }
  },
  {
    tag: 27,
    encode(regex, encode3) {
      encode3(["RegExp", regex.source, regex.flags]);
    }
  },
  {
    getTag(tag) {
      return tag.tag;
    },
    encode(tag, encode3) {
      encode3(tag.value);
    }
  },
  {
    encode(arrayBuffer, encode3, makeRoom) {
      writeBuffer2(arrayBuffer, makeRoom);
    }
  },
  {
    getTag(typedArray) {
      if (typedArray.constructor === Uint8Array) {
        if (this.tagUint8Array || hasNodeBuffer2 && this.tagUint8Array !== false)
          return 64;
      }
    },
    encode(typedArray, encode3, makeRoom) {
      writeBuffer2(typedArray, makeRoom);
    }
  },
  typedArrayEncoder2(68, 1),
  typedArrayEncoder2(69, 2),
  typedArrayEncoder2(70, 4),
  typedArrayEncoder2(71, 8),
  typedArrayEncoder2(72, 1),
  typedArrayEncoder2(77, 2),
  typedArrayEncoder2(78, 4),
  typedArrayEncoder2(79, 8),
  typedArrayEncoder2(85, 4),
  typedArrayEncoder2(86, 8),
  {
    encode(sharedData, encode3) {
      let packedValues3 = sharedData.packedValues || [];
      let sharedStructures = sharedData.structures || [];
      if (packedValues3.values.length > 0) {
        target2[position4++] = 216;
        target2[position4++] = 51;
        writeArrayHeader2(4);
        let valuesArray = packedValues3.values;
        encode3(valuesArray);
        writeArrayHeader2(0);
        writeArrayHeader2(0);
        packedObjectMap = Object.create(sharedPackedObjectMap || null);
        for (let i = 0, l = valuesArray.length; i < l; i++) {
          packedObjectMap[valuesArray[i]] = i;
        }
      }
      if (sharedStructures) {
        targetView2.setUint32(position4, 3655335424);
        position4 += 3;
        let definitions = sharedStructures.slice(0);
        definitions.unshift(57344);
        definitions.push(new Tag2(sharedData.version, 1399353956));
        encode3(definitions);
      } else
        encode3(new Tag2(sharedData.version, 1399353956));
    }
  }
];
function typedArrayEncoder2(tag, size) {
  if (!isLittleEndianMachine4 && size > 1)
    tag -= 4;
  return {
    tag,
    encode: function writeExtBuffer(typedArray, encode3) {
      let length = typedArray.byteLength;
      let offset = typedArray.byteOffset || 0;
      let buffer = typedArray.buffer || typedArray;
      encode3(hasNodeBuffer2 ? Buffer3.from(buffer, offset, length) : new Uint8Array(buffer, offset, length));
    }
  };
}
function writeBuffer2(buffer, makeRoom) {
  let length = buffer.byteLength;
  if (length < 24) {
    target2[position4++] = 64 + length;
  } else if (length < 256) {
    target2[position4++] = 88;
    target2[position4++] = length;
  } else if (length < 65536) {
    target2[position4++] = 89;
    target2[position4++] = length >> 8;
    target2[position4++] = length & 255;
  } else {
    target2[position4++] = 90;
    targetView2.setUint32(position4, length);
    position4 += 4;
  }
  if (position4 + length >= target2.length) {
    makeRoom(position4 + length);
  }
  target2.set(buffer.buffer ? buffer : new Uint8Array(buffer), position4);
  position4 += length;
}
function insertIds2(serialized, idsToInsert) {
  let nextId;
  let distanceToMove = idsToInsert.length * 2;
  let lastEnd = serialized.length - distanceToMove;
  idsToInsert.sort((a, b) => a.offset > b.offset ? 1 : -1);
  for (let id = 0; id < idsToInsert.length; id++) {
    let referee = idsToInsert[id];
    referee.id = id;
    for (let position5 of referee.references) {
      serialized[position5++] = id >> 8;
      serialized[position5] = id & 255;
    }
  }
  while (nextId = idsToInsert.pop()) {
    let offset = nextId.offset;
    serialized.copyWithin(offset + distanceToMove, offset, lastEnd);
    distanceToMove -= 2;
    let position5 = offset + distanceToMove;
    serialized[position5++] = 216;
    serialized[position5++] = 28;
    lastEnd = offset;
  }
  return serialized;
}
function writeBundles2(start, encode3) {
  targetView2.setUint32(bundledStrings4.position + start, position4 - bundledStrings4.position - start + 1);
  let writeStrings = bundledStrings4;
  bundledStrings4 = null;
  encode3(writeStrings[0]);
  encode3(writeStrings[1]);
}
function addExtension4(extension) {
  if (extension.Class) {
    if (!extension.encode)
      throw new Error("Extension has no encode function");
    extensionClasses2.unshift(extension.Class);
    extensions2.unshift(extension);
  }
  addExtension3(extension);
}
var defaultEncoder2 = new Encoder2({ useRecords: false });
var encode2 = defaultEncoder2.encode;
var encodeAsIterable2 = defaultEncoder2.encodeAsIterable;
var encodeAsAsyncIterable2 = defaultEncoder2.encodeAsAsyncIterable;
var { NEVER: NEVER2, ALWAYS: ALWAYS2, DECIMAL_ROUND: DECIMAL_ROUND2, DECIMAL_FIT: DECIMAL_FIT2 } = FLOAT32_OPTIONS2;
var REUSE_BUFFER_MODE2 = 512;
var RESET_BUFFER_MODE2 = 1024;
var THROW_ON_ITERABLE2 = 2048;

// node_modules/@progrium/duplex/esm/codec/cbor.js
var CBORCodec = class {
  constructor(debug3 = false, extensions3) {
    Object.defineProperty(this, "debug", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.debug = debug3;
    if (extensions3) {
      extensions3.forEach(addExtension4);
    }
  }
  encoder(w) {
    return new CBOREncoder(w, this.debug);
  }
  decoder(r) {
    return new CBORDecoder(r, this.debug);
  }
};
var CBOREncoder = class {
  constructor(w, debug3 = false) {
    Object.defineProperty(this, "w", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "debug", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.w = w;
    this.debug = debug3;
  }
  async encode(v) {
    if (this.debug) {
      console.log("<<", v);
    }
    let buf = encode2(v);
    let nwritten = 0;
    while (nwritten < buf.length) {
      nwritten += await this.w.write(buf.subarray(nwritten));
    }
  }
};
var CBORDecoder = class {
  constructor(r, debug3 = false) {
    Object.defineProperty(this, "r", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "debug", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.r = r;
    this.debug = debug3;
  }
  async decode(len) {
    const buf = new Uint8Array(len);
    let bufread = 0;
    while (bufread < len) {
      const n = await this.r.read(buf.subarray(bufread));
      if (n === null) {
        return Promise.resolve(null);
      }
      bufread += n;
    }
    let v = decode2(buf);
    if (this.debug) {
      console.log(">>", v);
    }
    return Promise.resolve(v);
  }
};

// node_modules/@progrium/duplex/esm/buffer.js
function copy(src3, dst, off = 0) {
  off = Math.max(0, Math.min(off, dst.byteLength));
  const dstBytesAvailable = dst.byteLength - off;
  if (src3.byteLength > dstBytesAvailable) {
    src3 = src3.subarray(0, dstBytesAvailable);
  }
  dst.set(src3, off);
  return src3.byteLength;
}
var MIN_READ = 32 * 1024;
var MAX_SIZE = 2 ** 32 - 2;
var Buffer4 = class {
  constructor(ab) {
    Object.defineProperty(this, "_buf", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "_off", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this._buf = ab === void 0 ? new Uint8Array(0) : new Uint8Array(ab);
    this._off = 0;
  }
  /** Returns a slice holding the unread portion of the buffer.
   *
   * The slice is valid for use only until the next buffer modification (that
   * is, only until the next call to a method like `read()`, `write()`,
   * `reset()`, or `truncate()`). If `options.copy` is false the slice aliases the buffer content at
   * least until the next buffer modification, so immediate changes to the
   * slice will affect the result of future reads.
   * @param options Defaults to `{ copy: true }`
   */
  bytes(options = { copy: true }) {
    if (options.copy === false)
      return this._buf.subarray(this._off);
    return this._buf.slice(this._off);
  }
  /** Returns whether the unread portion of the buffer is empty. */
  empty() {
    return this._buf.byteLength <= this._off;
  }
  /** A read only number of bytes of the unread portion of the buffer. */
  get length() {
    return this._buf.byteLength - this._off;
  }
  /** The read only capacity of the buffer's underlying byte slice, that is,
   * the total space allocated for the buffer's data. */
  get capacity() {
    return this._buf.buffer.byteLength;
  }
  /** Discards all but the first `n` unread bytes from the buffer but
   * continues to use the same allocated storage. It throws if `n` is
   * negative or greater than the length of the buffer. */
  truncate(n) {
    if (n === 0) {
      this.reset();
      return;
    }
    if (n < 0 || n > this.length) {
      throw Error("bytes.Buffer: truncation out of range");
    }
    this._reslice(this._off + n);
  }
  reset() {
    this._reslice(0);
    this._off = 0;
  }
  _tryGrowByReslice(n) {
    const l = this._buf.byteLength;
    if (n <= this.capacity - l) {
      this._reslice(l + n);
      return l;
    }
    return -1;
  }
  _reslice(len) {
    this._buf = new Uint8Array(this._buf.buffer, 0, len);
  }
  /** Reads the next `p.length` bytes from the buffer or until the buffer is
   * drained. Returns the number of bytes read. If the buffer has no data to
   * return, the return is EOF (`null`). */
  readSync(p) {
    if (this.empty()) {
      this.reset();
      if (p.byteLength === 0) {
        return 0;
      }
      return null;
    }
    const nread = copy(this._buf.subarray(this._off), p);
    this._off += nread;
    return nread;
  }
  /** Reads the next `p.length` bytes from the buffer or until the buffer is
   * drained. Resolves to the number of bytes read. If the buffer has no
   * data to return, resolves to EOF (`null`).
   *
   * NOTE: This methods reads bytes synchronously; it's provided for
   * compatibility with `Reader` interfaces.
   */
  read(p) {
    const rr = this.readSync(p);
    return Promise.resolve(rr);
  }
  writeSync(p) {
    const m = this._grow(p.byteLength);
    return copy(p, this._buf, m);
  }
  /** NOTE: This methods writes bytes synchronously; it's provided for
   * compatibility with `Writer` interface. */
  write(p) {
    const n = this.writeSync(p);
    return Promise.resolve(n);
  }
  _grow(n) {
    const m = this.length;
    if (m === 0 && this._off !== 0) {
      this.reset();
    }
    const i = this._tryGrowByReslice(n);
    if (i >= 0) {
      return i;
    }
    const c = this.capacity;
    if (n <= Math.floor(c / 2) - m) {
      copy(this._buf.subarray(this._off), this._buf);
    } else if (c + n > MAX_SIZE) {
      throw new Error("The buffer cannot be grown beyond the maximum size.");
    } else {
      const buf = new Uint8Array(Math.min(2 * c + n, MAX_SIZE));
      copy(this._buf.subarray(this._off), buf);
      this._buf = buf;
    }
    this._off = 0;
    this._reslice(Math.min(m + n, MAX_SIZE));
    return m;
  }
  /** Grows the buffer's capacity, if necessary, to guarantee space for
   * another `n` bytes. After `.grow(n)`, at least `n` bytes can be written to
   * the buffer without another allocation. If `n` is negative, `.grow()` will
   * throw. If the buffer can't grow it will throw an error.
   *
   * Based on Go Lang's
   * [Buffer.Grow](https://golang.org/pkg/bytes/_buffer.Grow). */
  grow(n) {
    if (n < 0) {
      throw Error("Buffer.grow: negative count");
    }
    const m = this._grow(n);
    this._reslice(m);
  }
  /** Reads data from `r` until EOF (`null`) and appends it to the buffer,
   * growing the buffer as needed. It resolves to the number of bytes read.
   * If the buffer becomes too large, `.readFrom()` will reject with an error.
   *
   * Based on Go Lang's
   * [Buffer.ReadFrom](https://golang.org/pkg/bytes/_buffer.ReadFrom). */
  async readFrom(r) {
    let n = 0;
    const tmp = new Uint8Array(MIN_READ);
    while (true) {
      const shouldGrow = this.capacity - this.length < MIN_READ;
      const buf = shouldGrow ? tmp : new Uint8Array(this._buf.buffer, this.length);
      const nread = await r.read(buf);
      if (nread === null) {
        return n;
      }
      if (shouldGrow)
        this.writeSync(buf.subarray(0, nread));
      else
        this._reslice(this.length + nread);
      n += nread;
    }
  }
  /** Reads data from `r` until EOF (`null`) and appends it to the buffer,
   * growing the buffer as needed. It returns the number of bytes read. If the
   * buffer becomes too large, `.readFromSync()` will throw an error.
   *
   * Based on Go Lang's
   * [Buffer.ReadFrom](https://golang.org/pkg/bytes/#Buffer.ReadFrom). */
  readFromSync(r) {
    let n = 0;
    const tmp = new Uint8Array(MIN_READ);
    while (true) {
      const shouldGrow = this.capacity - this.length < MIN_READ;
      const buf = shouldGrow ? tmp : new Uint8Array(this._buf.buffer, this.length);
      const nread = r.readSync(buf);
      if (nread === null) {
        return n;
      }
      if (shouldGrow)
        this.writeSync(buf.subarray(0, nread));
      else
        this._reslice(this.length + nread);
      n += nread;
    }
  }
};

// node_modules/@progrium/duplex/esm/codec/frame.js
var FrameCodec = class {
  constructor(codec) {
    Object.defineProperty(this, "codec", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.codec = codec;
  }
  encoder(w) {
    return new FrameEncoder(w, this.codec);
  }
  decoder(r) {
    return new FrameDecoder(r, this.codec.decoder(r));
  }
};
var FrameEncoder = class {
  constructor(w, codec) {
    Object.defineProperty(this, "w", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "codec", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.w = w;
    this.codec = codec;
  }
  async encode(v) {
    const data = new Buffer4();
    const enc = this.codec.encoder(data);
    await enc.encode(v);
    const lenPrefix = new DataView(new ArrayBuffer(4));
    lenPrefix.setUint32(0, data.length);
    const buf = new Uint8Array(data.length + 4);
    buf.set(new Uint8Array(lenPrefix.buffer), 0);
    buf.set(data.bytes(), 4);
    let nwritten = 0;
    while (nwritten < buf.length) {
      nwritten += await this.w.write(buf.subarray(nwritten));
    }
  }
};
var FrameDecoder = class {
  constructor(r, dec) {
    Object.defineProperty(this, "r", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "dec", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.r = r;
    this.dec = dec;
  }
  async decode(len) {
    const prefix = new Uint8Array(4);
    const prefixn = await this.r.read(prefix);
    if (prefixn === null) {
      return null;
    }
    const prefixv = new DataView(prefix.buffer);
    const size = prefixv.getUint32(0);
    return await this.dec.decode(size);
  }
};

// node_modules/@progrium/duplex/esm/rpc/client.js
var Client = class {
  constructor(session, codec) {
    Object.defineProperty(this, "session", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "codec", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.session = session;
    this.codec = codec;
  }
  async call(selector, args) {
    const ch = await this.session.open();
    try {
      const framer = new FrameCodec(this.codec);
      const enc = framer.encoder(ch);
      const dec = framer.decoder(ch);
      await enc.encode({ S: selector });
      await enc.encode(args);
      const header = await dec.decode();
      const resp = new Response2(ch, framer);
      resp.error = header.E;
      if (resp.error !== void 0 && resp.error !== null) {
        throw resp.error;
      }
      resp.value = await dec.decode();
      resp.continue = header.C;
      if (!resp.continue) {
        await ch.close();
      }
      return resp;
    } catch (e) {
      await ch.close();
      return Promise.reject(e);
    }
  }
};
function VirtualCaller(caller) {
  function pathBuilder(path, callable) {
    return new Proxy(Object.assign(() => {
    }, { path, callable }), {
      get(t, prop, rcvr) {
        if (prop.startsWith("__"))
          return Reflect.get(t, prop, rcvr);
        return pathBuilder(t.path ? `${t.path}.${prop}` : prop, t.callable);
      },
      apply(pc, thisArg, args = []) {
        return pc.callable(pc.path, args);
      }
    });
  }
  return pathBuilder("", caller.call.bind(caller));
}

// node_modules/@progrium/duplex/esm/rpc/handler.js
function HandlerFunc(fn) {
  return { respondRPC: fn };
}
function NotFoundHandler() {
  return HandlerFunc((r, c) => {
    r.return(new Error(`not found: ${c.selector}`));
  });
}
function cleanSelector(s) {
  if (s === "") {
    return "/";
  }
  if (s[0] != "/") {
    s = "/" + s;
  }
  s = s.replace(".", "/");
  return s.toLowerCase();
}
var RespondMux = class {
  constructor() {
    Object.defineProperty(this, "handlers", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.handlers = {};
  }
  async respondRPC(r, c) {
    const h = this.handler(c);
    await h.respondRPC(r, c);
  }
  handler(c) {
    const h = this.match(c.selector);
    if (!h) {
      return NotFoundHandler();
    }
    return h;
  }
  remove(selector) {
    selector = cleanSelector(selector);
    const h = this.match(selector);
    delete this.handlers[selector];
    return h || null;
  }
  match(selector) {
    selector = cleanSelector(selector);
    if (this.handlers.hasOwnProperty(selector)) {
      return this.handlers[selector];
    }
    const patterns = Object.keys(this.handlers).filter((pattern) => pattern.endsWith("/"));
    patterns.sort((a, b) => b.length - a.length);
    for (const pattern of patterns) {
      if (selector.startsWith(pattern)) {
        const handler = this.handlers[pattern];
        const matcher = handler;
        if (matcher.match && matcher.match instanceof Function) {
          return matcher.match(selector.slice(pattern.length));
        }
        return handler;
      }
    }
    return null;
  }
  handle(selector, handler) {
    if (selector === "") {
      throw "handle: invalid selector";
    }
    let pattern = cleanSelector(selector);
    const matcher = handler;
    if (matcher["match"] && matcher["match"] instanceof Function && !pattern.endsWith("/")) {
      pattern = pattern + "/";
    }
    if (!handler) {
      throw "handle: invalid handler";
    }
    if (this.match(pattern)) {
      throw "handle: selector already registered";
    }
    this.handlers[pattern] = handler;
  }
};

// node_modules/@progrium/duplex/esm/rpc/responder.js
async function Respond(ch, codec, handler) {
  const framer = new FrameCodec(codec);
  const dec = framer.decoder(ch);
  const callHeader = await dec.decode();
  const call = new Call(callHeader.S, ch, dec);
  call.caller = new Client(ch.session, codec);
  const respHeader = new ResponseHeader();
  const resp = new responder(ch, framer, respHeader);
  if (!handler) {
    handler = new RespondMux();
  }
  await handler.respondRPC(resp, call);
  if (!resp.responded) {
    await resp.return(null);
  }
  return Promise.resolve();
}
var responder = class {
  constructor(ch, codec, header) {
    Object.defineProperty(this, "header", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "ch", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "codec", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "responded", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.ch = ch;
    this.codec = codec;
    this.header = header;
    this.responded = false;
  }
  send(v) {
    return this.codec.encoder(this.ch).encode(v);
  }
  return(v) {
    return this.respond(v, false);
  }
  async continue(v) {
    await this.respond(v, true);
    return this.ch;
  }
  async respond(v, continue_) {
    this.responded = true;
    this.header.C = continue_;
    if (v instanceof Error) {
      this.header.E = v.message;
      v = null;
    }
    await this.send(this.header);
    await this.send(v);
    if (!continue_) {
      await this.ch.close();
    }
    return Promise.resolve();
  }
};

// node_modules/@progrium/duplex/esm/rpc/mod.js
var Call = class {
  constructor(selector, channel, decoder3) {
    Object.defineProperty(this, "selector", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "channel", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "caller", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "decoder", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.selector = selector;
    this.channel = channel;
    this.decoder = decoder3;
  }
  receive() {
    return this.decoder.decode();
  }
};
var ResponseHeader = class {
  constructor() {
    Object.defineProperty(this, "E", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "C", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.E = void 0;
    this.C = false;
  }
};
var Response2 = class {
  constructor(channel, codec) {
    Object.defineProperty(this, "error", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "continue", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "value", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "channel", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "codec", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.channel = channel;
    this.codec = codec;
    this.error = void 0;
    this.continue = false;
  }
  send(v) {
    return this.codec.encoder(this.channel).encode(v);
  }
  receive() {
    return this.codec.decoder(this.channel).decode();
  }
};

// node_modules/@progrium/duplex/esm/fn/proxy.js
function methodProxy(caller) {
  function pathBuilder(path, callable) {
    return new Proxy(Object.assign(() => {
    }, { path, callable }), {
      get(t, prop, rcvr) {
        if (prop.startsWith("__"))
          return Reflect.get(t, prop, rcvr);
        return pathBuilder(t.path ? `${t.path}.${prop}` : prop, t.callable);
      },
      apply(pc, thisArg, args = []) {
        return pc.callable(pc.path, args);
      }
    });
  }
  return pathBuilder("", caller.call.bind(caller));
}

// node_modules/@progrium/duplex/esm/peer/peer.js
var Peer = class {
  constructor(session, codec) {
    Object.defineProperty(this, "session", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "caller", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "codec", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "responder", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.session = session;
    this.codec = codec;
    this.caller = new Client(session, codec);
    this.responder = new RespondMux();
  }
  close() {
    return this.session.close();
  }
  async respond() {
    while (true) {
      const ch = await this.session.accept();
      if (ch === null) {
        break;
      }
      Respond(ch, this.codec, this.responder);
    }
  }
  async call(selector, params) {
    return this.caller.call(selector, params);
  }
  handle(selector, handler) {
    this.responder.handle(selector, handler);
  }
  respondRPC(r, c) {
    this.responder.respondRPC(r, c);
  }
  proxy() {
    return methodProxy(this.caller);
  }
  // deprecated
  virtualize() {
    return VirtualCaller(this.caller);
  }
};

// node_modules/@progrium/duplex/esm/mux/codec/message.js
var OpenID = 100;
var OpenConfirmID = 101;
var OpenFailureID = 102;
var WindowAdjustID = 103;
var DataID = 104;
var EofID = 105;
var CloseID = 106;
var payloadSizes = /* @__PURE__ */ new Map([
  [OpenID, 12],
  [OpenConfirmID, 16],
  [OpenFailureID, 4],
  [WindowAdjustID, 8],
  [DataID, 8],
  [EofID, 4],
  [CloseID, 4]
]);

// node_modules/@progrium/duplex/esm/mux/codec/encoder.js
var Encoder3 = class {
  constructor(w) {
    Object.defineProperty(this, "w", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.w = w;
  }
  async encode(m) {
    if (debug2.messages) {
      console.log("<<ENC", m);
    }
    const buf = Marshal(m);
    if (debug2.bytes) {
      console.log("<<ENC", buf);
    }
    let nwritten = 0;
    while (nwritten < buf.length) {
      nwritten += await this.w.write(buf.subarray(nwritten));
    }
    return nwritten;
  }
};
function Marshal(obj) {
  if (obj.ID === CloseID) {
    const m = obj;
    const data = new DataView(new ArrayBuffer(5));
    data.setUint8(0, m.ID);
    data.setUint32(1, m.channelID);
    return new Uint8Array(data.buffer);
  }
  if (obj.ID === DataID) {
    const m = obj;
    const data = new DataView(new ArrayBuffer(9));
    data.setUint8(0, m.ID);
    data.setUint32(1, m.channelID);
    data.setUint32(5, m.length);
    const buf = new Uint8Array(9 + m.length);
    buf.set(new Uint8Array(data.buffer), 0);
    buf.set(m.data, 9);
    return buf;
  }
  if (obj.ID === EofID) {
    const m = obj;
    const data = new DataView(new ArrayBuffer(5));
    data.setUint8(0, m.ID);
    data.setUint32(1, m.channelID);
    return new Uint8Array(data.buffer);
  }
  if (obj.ID === OpenID) {
    const m = obj;
    const data = new DataView(new ArrayBuffer(13));
    data.setUint8(0, m.ID);
    data.setUint32(1, m.senderID);
    data.setUint32(5, m.windowSize);
    data.setUint32(9, m.maxPacketSize);
    return new Uint8Array(data.buffer);
  }
  if (obj.ID === OpenConfirmID) {
    const m = obj;
    const data = new DataView(new ArrayBuffer(17));
    data.setUint8(0, m.ID);
    data.setUint32(1, m.channelID);
    data.setUint32(5, m.senderID);
    data.setUint32(9, m.windowSize);
    data.setUint32(13, m.maxPacketSize);
    return new Uint8Array(data.buffer);
  }
  if (obj.ID === OpenFailureID) {
    const m = obj;
    const data = new DataView(new ArrayBuffer(5));
    data.setUint8(0, m.ID);
    data.setUint32(1, m.channelID);
    return new Uint8Array(data.buffer);
  }
  if (obj.ID === WindowAdjustID) {
    const m = obj;
    const data = new DataView(new ArrayBuffer(9));
    data.setUint8(0, m.ID);
    data.setUint32(1, m.channelID);
    data.setUint32(5, m.additionalBytes);
    return new Uint8Array(data.buffer);
  }
  throw `marshal of unknown type: ${obj}`;
}

// node_modules/@progrium/duplex/esm/mux/util.js
function concat(list, totalLength) {
  const buf = new Uint8Array(totalLength);
  let offset = 0;
  list.forEach((el) => {
    buf.set(el, offset);
    offset += el.length;
  });
  return buf;
}
var queue = class {
  constructor() {
    Object.defineProperty(this, "q", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "waiters", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "closed", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.q = [];
    this.waiters = [];
    this.closed = false;
  }
  push(obj) {
    if (this.closed)
      throw "closed queue";
    if (this.waiters.length > 0) {
      const waiter = this.waiters.shift();
      if (waiter)
        waiter(obj);
      return;
    }
    this.q.push(obj);
  }
  shift() {
    if (this.closed)
      return Promise.resolve(null);
    return new Promise((resolve) => {
      if (this.q.length > 0) {
        resolve(this.q.shift() || null);
        return;
      }
      this.waiters.push(resolve);
    });
  }
  close() {
    if (this.closed)
      return;
    this.closed = true;
    this.waiters.forEach((waiter) => {
      waiter(null);
    });
  }
};
var ReadBuffer = class {
  constructor() {
    Object.defineProperty(this, "gotEOF", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "readBuf", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "readers", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.readBuf = new Uint8Array(0);
    this.gotEOF = false;
    this.readers = [];
  }
  read(p) {
    return new Promise((resolve) => {
      let tryRead = () => {
        if (this.readBuf === void 0) {
          resolve(null);
          return;
        }
        if (this.readBuf.length == 0) {
          if (this.gotEOF) {
            this.readBuf = void 0;
            resolve(null);
            return;
          }
          this.readers.push(tryRead);
          return;
        }
        const data = this.readBuf.slice(0, p.length);
        this.readBuf = this.readBuf.slice(data.length);
        if (this.readBuf.length == 0 && this.gotEOF) {
          this.readBuf = void 0;
        }
        p.set(data);
        resolve(data.length);
      };
      tryRead();
    });
  }
  write(p) {
    if (this.readBuf) {
      this.readBuf = concat([this.readBuf, p], this.readBuf.length + p.length);
    }
    while (!this.readBuf || this.readBuf.length > 0) {
      let reader = this.readers.shift();
      if (!reader)
        break;
      reader();
    }
    return Promise.resolve(p.length);
  }
  eof() {
    this.gotEOF = true;
    this.flushReaders();
  }
  close() {
    this.readBuf = void 0;
    this.flushReaders();
  }
  flushReaders() {
    while (true) {
      const reader = this.readers.shift();
      if (!reader)
        return;
      reader();
    }
  }
};

// node_modules/@progrium/duplex/esm/mux/codec/decoder.js
var Decoder3 = class {
  constructor(r) {
    Object.defineProperty(this, "r", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.r = r;
  }
  async decode() {
    const packet = await readPacket(this.r);
    if (packet === null) {
      return Promise.resolve(null);
    }
    if (debug2.bytes) {
      console.log(">>DEC", packet);
    }
    const msg = Unmarshal(packet);
    if (debug2.messages) {
      console.log(">>DEC", msg);
    }
    return msg;
  }
};
async function readPacket(r) {
  const head = new Uint8Array(1);
  const headn = await r.read(head);
  if (headn === null) {
    return Promise.resolve(null);
  }
  const msgID = head[0];
  const size = payloadSizes.get(msgID);
  if (size === void 0 || msgID < OpenID || msgID > CloseID) {
    return Promise.reject(`bad packet: ${msgID}`);
  }
  const rest = new Uint8Array(size);
  const restn = await r.read(rest);
  if (restn === null) {
    return Promise.reject("unexpected EOF reading packet");
  }
  if (msgID === DataID) {
    const view = new DataView(rest.buffer);
    const datasize = view.getUint32(4);
    let dataread = 0;
    const chunks = [];
    while (dataread < datasize) {
      const chunk = new Uint8Array(datasize - dataread);
      const chunkread = await r.read(chunk);
      if (chunkread === null) {
        return Promise.reject(`unexpected EOF reading data chunk`);
      }
      dataread += chunkread;
      chunks.push(chunk.slice(0, chunkread));
    }
    return concat([head, rest, ...chunks], 1 + rest.length + datasize);
  }
  return concat([head, rest], rest.length + 1);
}
function Unmarshal(packet) {
  const data = new DataView(packet.buffer);
  switch (packet[0]) {
    case CloseID:
      return {
        ID: packet[0],
        channelID: data.getUint32(1)
      };
    case DataID:
      let dataLength = data.getUint32(5);
      let rest = new Uint8Array(packet.buffer.slice(9));
      return {
        ID: packet[0],
        channelID: data.getUint32(1),
        length: dataLength,
        data: rest
      };
    case EofID:
      return {
        ID: packet[0],
        channelID: data.getUint32(1)
      };
    case OpenID:
      return {
        ID: packet[0],
        senderID: data.getUint32(1),
        windowSize: data.getUint32(5),
        maxPacketSize: data.getUint32(9)
      };
    case OpenConfirmID:
      return {
        ID: packet[0],
        channelID: data.getUint32(1),
        senderID: data.getUint32(5),
        windowSize: data.getUint32(9),
        maxPacketSize: data.getUint32(13)
      };
    case OpenFailureID:
      return {
        ID: packet[0],
        channelID: data.getUint32(1)
      };
    case WindowAdjustID:
      return {
        ID: packet[0],
        channelID: data.getUint32(1),
        additionalBytes: data.getUint32(5)
      };
    default:
      throw `unmarshal of unknown type: ${packet[0]}`;
  }
}

// node_modules/@progrium/duplex/esm/mux/codec/mod.js
var debug2 = {
  messages: false,
  bytes: false
};

// node_modules/@progrium/duplex/esm/mux/session/session.js
var minPacketLength = 9;
var maxPacketLength = Number.MAX_VALUE;
var Session = class {
  constructor(conn) {
    Object.defineProperty(this, "conn", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "channels", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "incoming", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "enc", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "dec", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "done", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "closed", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.conn = conn;
    this.enc = new Encoder3(conn);
    this.dec = new Decoder3(conn);
    this.channels = [];
    this.incoming = new queue();
    this.done = this.loop();
    this.closed = false;
  }
  async open() {
    const ch = this.newChannel();
    ch.maxIncomingPayload = channelMaxPacket;
    await this.enc.encode({
      ID: OpenID,
      windowSize: ch.myWindow,
      maxPacketSize: ch.maxIncomingPayload,
      senderID: ch.localId
    });
    if (await ch.ready.shift()) {
      return ch;
    }
    throw "failed to open";
  }
  accept() {
    return this.incoming.shift();
  }
  async close() {
    for (const ids of Object.keys(this.channels)) {
      const id = parseInt(ids);
      if (this.channels[id] !== void 0) {
        this.channels[id].shutdown();
      }
    }
    this.conn.close();
    this.closed = true;
    await this.done;
  }
  async loop() {
    try {
      while (true) {
        const msg = await this.dec.decode();
        if (msg === null) {
          this.close();
          return;
        }
        if (msg.ID === OpenID) {
          await this.handleOpen(msg);
          continue;
        }
        const cmsg = msg;
        const ch = this.getCh(cmsg.channelID);
        if (ch === void 0) {
          if (this.closed) {
            return;
          }
          console.warn(`invalid channel (${cmsg.channelID}) on op ${cmsg.ID}`);
          continue;
        }
        await ch.handle(cmsg);
      }
    } catch (e) {
      if (e.message && e.message.contains && e.message.contains("Connection reset by peer")) {
        return;
      }
      throw e;
    }
  }
  async handleOpen(msg) {
    if (msg.maxPacketSize < minPacketLength || msg.maxPacketSize > maxPacketLength) {
      await this.enc.encode({
        ID: OpenFailureID,
        channelID: msg.senderID
      });
      return;
    }
    const c = this.newChannel();
    c.remoteId = msg.senderID;
    c.maxRemotePayload = msg.maxPacketSize;
    c.remoteWin = msg.windowSize;
    c.maxIncomingPayload = channelMaxPacket;
    this.incoming.push(c);
    await this.enc.encode({
      ID: OpenConfirmID,
      channelID: c.remoteId,
      senderID: c.localId,
      windowSize: c.myWindow,
      maxPacketSize: c.maxIncomingPayload
    });
  }
  newChannel() {
    const ch = new Channel(this);
    ch.remoteWin = 0;
    ch.myWindow = channelWindowSize;
    ch.localId = this.addCh(ch);
    return ch;
  }
  getCh(id) {
    const ch = this.channels[id];
    if (ch && ch.localId !== id) {
      console.log("bad ids:", id, ch.localId, ch.remoteId);
    }
    return ch;
  }
  addCh(ch) {
    this.channels.forEach((v, i) => {
      if (v === void 0) {
        this.channels[i] = ch;
        return i;
      }
    });
    this.channels.push(ch);
    return this.channels.length - 1;
  }
  rmCh(id) {
    delete this.channels[id];
  }
};

// node_modules/@progrium/duplex/esm/mux/session/channel.js
var channelMaxPacket = 1 << 24;
var channelWindowSize = 64 * channelMaxPacket;
var Channel = class {
  constructor(sess) {
    Object.defineProperty(this, "localId", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "remoteId", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "maxIncomingPayload", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "maxRemotePayload", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "session", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "ready", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "sentEOF", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "sentClose", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "remoteWin", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "myWindow", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "readBuf", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "writers", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.localId = 0;
    this.remoteId = 0;
    this.maxIncomingPayload = 0;
    this.maxRemotePayload = 0;
    this.sentEOF = false;
    this.sentClose = false;
    this.remoteWin = 0;
    this.myWindow = 0;
    this.ready = new queue();
    this.session = sess;
    this.writers = [];
    this.readBuf = new ReadBuffer();
  }
  get id() {
    return this.localId;
  }
  async read(p) {
    let n = await this.readBuf.read(p);
    if (n !== null) {
      try {
        await this.adjustWindow(n);
      } catch (e) {
        if (e !== "EOF" && e.name !== "BadResource") {
          throw e;
        }
      }
    }
    return n;
  }
  write(p) {
    if (this.sentEOF) {
      return Promise.reject("EOF");
    }
    return new Promise((resolve, reject) => {
      let n = 0;
      const tryWrite = () => {
        if (this.sentEOF || this.sentClose) {
          reject("EOF");
          return;
        }
        if (p.byteLength == 0) {
          resolve(n);
          return;
        }
        const space = Math.min(this.maxRemotePayload, p.length);
        const reserved = this.reserveWindow(space);
        if (reserved == 0) {
          this.writers.push(tryWrite);
          return;
        }
        const toSend = p.slice(0, reserved);
        this.send({
          ID: DataID,
          channelID: this.remoteId,
          length: toSend.length,
          data: toSend
        }).then(() => {
          n += toSend.length;
          p = p.slice(toSend.length);
          if (p.length == 0) {
            resolve(n);
            return;
          }
          this.writers.push(tryWrite);
        });
      };
      tryWrite();
    });
  }
  reserveWindow(win) {
    if (this.remoteWin < win) {
      win = this.remoteWin;
    }
    this.remoteWin -= win;
    return win;
  }
  addWindow(win) {
    this.remoteWin += win;
    while (this.remoteWin > 0) {
      const writer = this.writers.shift();
      if (!writer)
        break;
      writer();
    }
  }
  async closeWrite() {
    this.sentEOF = true;
    await this.send({
      ID: EofID,
      channelID: this.remoteId
    });
    this.writers.forEach((writer) => writer());
    this.writers = [];
  }
  async close() {
    this.readBuf.eof();
    if (!this.sentClose) {
      await this.send({
        ID: CloseID,
        channelID: this.remoteId
      });
      this.sentClose = true;
      while (await this.ready.shift() !== null) {
      }
      return;
    }
    this.shutdown();
  }
  shutdown() {
    this.readBuf.close();
    this.writers.forEach((writer) => writer());
    this.ready.close();
    this.session.rmCh(this.localId);
  }
  async adjustWindow(n) {
    this.myWindow += n;
    await this.send({
      ID: WindowAdjustID,
      channelID: this.remoteId,
      additionalBytes: n
    });
  }
  send(msg) {
    if (this.sentClose) {
      throw "EOF";
    }
    this.sentClose = msg.ID === CloseID;
    return this.session.enc.encode(msg);
  }
  handle(msg) {
    if (msg.ID === DataID) {
      this.handleData(msg);
      return;
    }
    if (msg.ID === CloseID) {
      this.close();
      return;
    }
    if (msg.ID === EofID) {
      this.readBuf.eof();
    }
    if (msg.ID === OpenFailureID) {
      this.session.rmCh(msg.channelID);
      this.ready.push(false);
      return;
    }
    if (msg.ID === OpenConfirmID) {
      if (msg.maxPacketSize < minPacketLength || msg.maxPacketSize > maxPacketLength) {
        throw "invalid max packet size";
      }
      this.remoteId = msg.senderID;
      this.maxRemotePayload = msg.maxPacketSize;
      this.addWindow(msg.windowSize);
      this.ready.push(true);
      return;
    }
    if (msg.ID === WindowAdjustID) {
      this.addWindow(msg.additionalBytes);
    }
  }
  handleData(msg) {
    if (msg.length > this.maxIncomingPayload) {
      throw "incoming packet exceeds maximum payload size";
    }
    if (this.myWindow < msg.length) {
      throw "remote side wrote too much";
    }
    this.myWindow -= msg.length;
    this.readBuf.write(msg.data);
  }
};

// node_modules/@progrium/duplex/esm/_dnt.shims.js
var import_ws = __toESM(require_browser(), 1);
var import_ws2 = __toESM(require_browser(), 1);
var dntGlobals = {
  WebSocket: import_ws.default
};
var dntGlobalThis = createMergeProxy(globalThis, dntGlobals);
function createMergeProxy(baseObj, extObj) {
  return new Proxy(baseObj, {
    get(_target, prop, _receiver) {
      if (prop in extObj) {
        return extObj[prop];
      } else {
        return baseObj[prop];
      }
    },
    set(_target, prop, value) {
      if (prop in extObj) {
        delete extObj[prop];
      }
      baseObj[prop] = value;
      return true;
    },
    deleteProperty(_target, prop) {
      let success = false;
      if (prop in extObj) {
        delete extObj[prop];
        success = true;
      }
      if (prop in baseObj) {
        delete baseObj[prop];
        success = true;
      }
      return success;
    },
    ownKeys(_target) {
      const baseKeys = Reflect.ownKeys(baseObj);
      const extKeys = Reflect.ownKeys(extObj);
      const extKeysSet = new Set(extKeys);
      return [...baseKeys.filter((k) => !extKeysSet.has(k)), ...extKeys];
    },
    defineProperty(_target, prop, desc) {
      if (prop in extObj) {
        delete extObj[prop];
      }
      Reflect.defineProperty(baseObj, prop, desc);
      return true;
    },
    getOwnPropertyDescriptor(_target, prop) {
      if (prop in extObj) {
        return Reflect.getOwnPropertyDescriptor(extObj, prop);
      } else {
        return Reflect.getOwnPropertyDescriptor(baseObj, prop);
      }
    },
    has(_target, prop) {
      return prop in extObj || prop in baseObj;
    }
  });
}

// node_modules/@progrium/duplex/esm/transport/messageport.js
var Conn = class {
  constructor(port) {
    Object.defineProperty(this, "port", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "waiters", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "chunks", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "isClosed", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.isClosed = false;
    this.waiters = [];
    this.chunks = [];
    this.port = port;
    this.port.onmessage = (event) => {
      const chunk = new Uint8Array(event.data);
      this.chunks.push(chunk);
      if (this.waiters.length > 0) {
        const waiter = this.waiters.shift();
        if (waiter)
          waiter();
      }
    };
  }
  read(p) {
    return new Promise((resolve) => {
      var tryRead = () => {
        if (this.isClosed) {
          resolve(null);
          return;
        }
        if (this.chunks.length === 0) {
          this.waiters.push(tryRead);
          return;
        }
        let written = 0;
        while (written < p.length) {
          const chunk = this.chunks.shift();
          if (chunk === null || chunk === void 0) {
            resolve(written);
            return;
          }
          const buf = chunk.slice(0, p.length - written);
          p.set(buf, written);
          written += buf.length;
          if (chunk.length > buf.length) {
            const restchunk = chunk.slice(buf.length);
            this.chunks.unshift(restchunk);
          }
        }
        resolve(written);
        return;
      };
      tryRead();
    });
  }
  write(p) {
    this.port.postMessage(p, [p.buffer]);
    return Promise.resolve(p.byteLength);
  }
  close() {
    if (this.isClosed)
      return;
    this.isClosed = true;
    this.waiters.forEach((waiter) => waiter());
    this.port.close();
  }
};

// handle.js
var WanixHandle2 = class {
  constructor(port) {
    const sess = new Session(new Conn(port));
    this.peer = new Peer(sess, new CBORCodec());
  }
  async readDir(name) {
    return (await this.peer.call("ReadDir", [name])).value;
  }
  async makeDir(name) {
    await this.peer.call("Mkdir", [name]);
  }
  async makeDirAll(name) {
    await this.peer.call("MkdirAll", [name]);
  }
  async bind(name, newname) {
    await this.peer.call("Bind", [name, newname]);
  }
  async unbind(name, newname) {
    await this.peer.call("Unbind", [name, newname]);
  }
  async readFile(name) {
    return (await this.peer.call("ReadFile", [name])).value;
  }
  // not sure if readFile approach is good, but this is an option for now
  async readFile2(name) {
    const rd = await this.openReadable(name);
    const response = new Response(rd);
    return new Uint8Array(await response.arrayBuffer());
  }
  async readText(name) {
    return new TextDecoder().decode(await this.readFile(name));
  }
  async waitFor(name, timeoutMs = 1e3) {
    await this.peer.call("WaitFor", [name, timeoutMs]);
  }
  async stat(name) {
    return (await this.peer.call("Stat", [name])).value;
  }
  async writeFile(name, contents) {
    if (typeof contents === "string") {
      contents = new TextEncoder().encode(contents);
    }
    return (await this.peer.call("WriteFile", [name, contents])).value;
  }
  async appendFile(name, contents) {
    if (typeof contents === "string") {
      contents = new TextEncoder().encode(contents);
    }
    return (await this.peer.call("AppendFile", [name, contents])).value;
  }
  async rename(oldname, newname) {
    await this.peer.call("Rename", [oldname, newname]);
  }
  async copy(oldname, newname) {
    await this.peer.call("Copy", [oldname, newname]);
  }
  async remove(name) {
    await this.peer.call("Remove", [name]);
  }
  async removeAll(name) {
    await this.peer.call("RemoveAll", [name]);
  }
  async truncate(name, size) {
    await this.peer.call("Truncate", [name, size]);
  }
  async open(name) {
    return (await this.peer.call("Open", [name])).value;
  }
  async read(fd, count) {
    return (await this.peer.call("Read", [fd, count])).value;
  }
  async write(fd, data) {
    return (await this.peer.call("Write", [fd, data])).value;
  }
  async close(fd) {
    return (await this.peer.call("Close", [fd])).value;
  }
  async sync(fd) {
    return (await this.peer.call("Sync", [fd])).value;
  }
  async openReadable(name) {
    const fd = await this.open(name);
    return this.readable(fd);
  }
  async openWritable(name) {
    const fd = await this.open(name);
    return this.writable(fd);
  }
  writable(fd) {
    const self = this;
    return new WritableStream({
      write(chunk) {
        return self.write(fd, chunk);
      }
    });
  }
  readable(fd) {
    const self = this;
    return new ReadableStream({
      async pull(controller) {
        const data = await self.read(fd, 1024);
        if (data === null) {
          controller.close();
        }
        controller.enqueue(data);
      }
    });
  }
};
export {
  CallBuffer,
  ConsoleStdout,
  Directory2 as Directory,
  DirectoryHandle,
  EmptyFile,
  File2 as File,
  FileHandle,
  OpenEmptyFile,
  OpenFile2 as OpenFile,
  PreopenDirectory2 as PreopenDirectory,
  WASI,
  WASIProcExit,
  WanixHandle2 as WanixHandle,
  applyPatchPollOneoff
};
