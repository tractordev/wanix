var __defProp = Object.defineProperty;
var __export = (target2, all) => {
  for (var name in all)
    __defProp(target2, name, { get: all[name], enumerable: true });
};

// codec/json.ts
var JSONCodec = class {
  constructor(debug2 = false) {
    this.debug = debug2;
  }
  encoder(w) {
    return new JSONEncoder(w, this.debug);
  }
  decoder(r) {
    return new JSONDecoder(r, this.debug);
  }
};
var JSONEncoder = class {
  constructor(w, debug2 = false) {
    this.w = w;
    this.enc = new TextEncoder();
    this.debug = debug2;
  }
  async encode(v) {
    if (this.debug) {
      console.log("<<", v);
    }
    let buf = this.enc.encode(JSON.stringify(v));
    let nwritten = 0;
    while (nwritten < buf.length) {
      nwritten += await this.w.write(buf.subarray(nwritten));
    }
  }
};
var JSONDecoder = class {
  constructor(r, debug2 = false) {
    this.r = r;
    this.dec = new TextDecoder();
    this.debug = debug2;
  }
  async decode(len) {
    const buf = new Uint8Array(len);
    const bufn = await this.r.read(buf);
    if (bufn === null) {
      return Promise.resolve(null);
    }
    let v = JSON.parse(this.dec.decode(buf));
    if (this.debug) {
      console.log(">>", v);
    }
    return Promise.resolve(v);
  }
};

// vnd/cbor-x-1.4.1/decode.js
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
var Decoder = class {
  constructor(options2) {
    if (options2) {
      if ((options2.keyMap || options2._keyMap) && !options2.useRecords) {
        options2.useRecords = false;
        options2.mapsAsObjects = true;
      }
      if (options2.useRecords === false && options2.mapsAsObjects === void 0)
        options2.mapsAsObjects = true;
      if (options2.getStructures)
        options2.getShared = options2.getStructures;
      if (options2.getShared && !options2.structures)
        (options2.structures = []).uninitialized = true;
      if (options2.keyMap) {
        this.mapKey = /* @__PURE__ */ new Map();
        for (let [k, v] of Object.entries(options2.keyMap))
          this.mapKey.set(v, k);
      }
    }
    Object.assign(this, options2);
  }
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
        return this ? this.decode(source, end) : Decoder.prototype.decode.call(defaultOptions, source, end);
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
    if (this instanceof Decoder) {
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
          case 3:
            throw new Error("Indefinite length not supported for byte or text strings");
          case 4:
            let array = [];
            let value, i = 0;
            while ((value = read()) != STOP_CODE) {
              array[i++] = value;
            }
            return majorType == 4 ? array : majorType == 3 ? array.join("") : Buffer.concat(array);
          case 5:
            let key;
            if (currentDecoder.mapsAsObjects) {
              let object = {};
              if (currentDecoder.keyMap)
                while ((key = read()) != STOP_CODE)
                  object[safeKey(currentDecoder.decodeKey(key))] = read();
              else
                while ((key = read()) != STOP_CODE)
                  object[safeKey(key)] = read();
              return object;
            } else {
              if (restoreMapsAsObject) {
                currentDecoder.mapsAsObjects = true;
                restoreMapsAsObject = false;
              }
              let map = /* @__PURE__ */ new Map();
              if (currentDecoder.keyMap)
                while ((key = read()) != STOP_CODE)
                  map.set(currentDecoder.decodeKey(key), read());
              else
                while ((key = read()) != STOP_CODE)
                  map.set(key, read());
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
      let array = new Array(token);
      for (let i = 0; i < token; i++)
        array[i] = read();
      return array;
    case 5:
      if (currentDecoder.mapsAsObjects) {
        let object = {};
        if (currentDecoder.keyMap)
          for (let i = 0; i < token; i++)
            object[safeKey(currentDecoder.decodeKey(read()))] = read();
        else
          for (let i = 0; i < token; i++)
            object[safeKey(read())] = read();
        return object;
      } else {
        if (restoreMapsAsObject) {
          currentDecoder.mapsAsObjects = true;
          restoreMapsAsObject = false;
        }
        let map = /* @__PURE__ */ new Map();
        if (currentDecoder.keyMap)
          for (let i = 0; i < token; i++)
            map.set(currentDecoder.decodeKey(read()), read());
        else
          for (let i = 0; i < token; i++)
            map.set(read(), read());
        return map;
      }
    case 6:
      if (token >= BUNDLED_STRINGS_ID) {
        let structure = currentStructures[token & 8191];
        if (structure) {
          if (!structure.read)
            structure.read = createStructureReader(structure);
          return structure.read();
        }
        if (token < 65536) {
          if (token == RECORD_INLINE_ID) {
            let length = readJustLength();
            let id = read();
            let structure2 = read();
            recordDefinition(id, structure2);
            let object = {};
            if (currentDecoder.keyMap)
              for (let i = 2; i < length; i++) {
                let key = currentDecoder.decodeKey(structure2[i - 2]);
                object[safeKey(key)] = read();
              }
            else
              for (let i = 2; i < length; i++) {
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
    if (this.slowReads++ >= 3) {
      let array = this.length == length ? this : this.slice(0, length);
      compiledReader = currentDecoder.keyMap ? new Function("r", "return {" + array.map((k) => currentDecoder.decodeKey(k)).map((k) => validName.test(k) ? safeKey(k) + ":r()" : "[" + JSON.stringify(k) + "]:r()").join(",") + "}") : new Function("r", "return {" + array.map((key) => validName.test(key) ? safeKey(key) + ":r()" : "[" + JSON.stringify(key) + "]:r()").join(",") + "}");
      if (this.compiledReader)
        compiledReader.next = this.compiledReader;
      compiledReader.propertyCount = length;
      this.compiledReader = compiledReader;
      return compiledReader(read);
    }
    let object = {};
    if (currentDecoder.keyMap)
      for (let i = 0; i < length; i++)
        object[safeKey(currentDecoder.decodeKey(this[i]))] = read();
    else
      for (let i = 0; i < length; i++) {
        object[safeKey(this[i])] = read();
      }
    return object;
  }
  structure.slowReads = 0;
  return readObject;
}
function safeKey(key) {
  return key === "__proto__" ? "__proto_" : key;
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
  return currentDecoder.copyBuffers ? Uint8Array.prototype.slice.call(src, position, position += length) : src.subarray(position, position += length);
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
  u8Array[3] = byte0 & 128 | (exponent >> 1) + 56;
  u8Array[2] = (byte0 & 7) << 5 | byte1 >> 3;
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
    value = BigInt(buffer[i]) + value << BigInt(8);
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
var packedTable = (read2) => {
  if (src[position++] != 132)
    throw new Error("Packed values structure must be followed by a 4 element array");
  let newPackedValues = read2();
  packedValues = packedValues ? newPackedValues.concat(packedValues.slice(newPackedValues.length)) : newPackedValues;
  packedValues.prefixes = read2();
  packedValues.suffixes = read2();
  return read2();
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
  throw new Error("No support for non-integer packed references yet");
};
currentExtensions[28] = (read2) => {
  if (!referenceMap) {
    referenceMap = /* @__PURE__ */ new Map();
    referenceMap.id = 0;
  }
  let id = referenceMap.id++;
  let token = src[position];
  let target2;
  if (token >> 5 == 4)
    target2 = [];
  else
    target2 = {};
  let refEntry = { target: target2 };
  referenceMap.set(id, refEntry);
  let targetProperties = read2();
  if (refEntry.used)
    return Object.assign(target2, targetProperties);
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
(currentExtensions[259] = (read2) => {
  if (currentDecoder.mapsAsObjects) {
    currentDecoder.mapsAsObjects = false;
    restoreMapsAsObject = true;
  }
  return read2();
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
  if (typeof TypedArray !== "function")
    TypedArray = null;
  let bytesPerElement = TypedArray.BYTES_PER_ELEMENT;
  for (let littleEndian = 0; littleEndian < 2; littleEndian++) {
    if (!littleEndian && bytesPerElement == 1)
      continue;
    let sizeShift = bytesPerElement == 2 ? 1 : bytesPerElement == 4 ? 2 : 3;
    currentExtensions[littleEndian ? tag : tag - 4] = bytesPerElement == 1 || littleEndian == isLittleEndianMachine ? (buffer) => {
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
function addExtension(extension) {
  currentExtensions[extension.tag] = extension.decode;
}
var mult10 = new Array(147);
for (let i = 0; i < 256; i++) {
  mult10[i] = +("1e" + Math.floor(45.15 - i * 0.30103));
}
var defaultDecoder = new Decoder({ useRecords: false });
var decode = defaultDecoder.decode;
var decodeMultiple = defaultDecoder.decodeMultiple;

// vnd/cbor-x-1.4.1/encode.js
var textEncoder;
try {
  textEncoder = new TextEncoder();
} catch (error) {
}
var extensions;
var extensionClasses;
var Buffer2 = globalThis.Buffer;
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
  constructor(options2) {
    super(options2);
    this.offset = 0;
    let typeBuffer;
    let start;
    let sharedStructures;
    let hasSharedUpdate;
    let structures;
    let referenceMap2;
    options2 = options2 || {};
    let encodeUtf8 = ByteArray.prototype.utf8Write ? function(string, position3, maxBytes) {
      return target.utf8Write(string, position3, maxBytes);
    } : textEncoder && textEncoder.encodeInto ? function(string, position3) {
      return textEncoder.encodeInto(string, target.subarray(position3)).written;
    } : false;
    let encoder = this;
    let hasSharedStructures = options2.structures || options2.saveStructures;
    let maxSharedStructures = options2.maxSharedStructures;
    if (maxSharedStructures == null)
      maxSharedStructures = hasSharedStructures ? 128 : 0;
    if (maxSharedStructures > 8190)
      throw new Error("Maximum maxSharedStructure is 8190");
    let isSequential = options2.sequential;
    if (isSequential) {
      maxSharedStructures = 0;
    }
    if (!this.structures)
      this.structures = [];
    if (this.saveStructures)
      this.saveShared = this.saveStructures;
    let samplingPackedValues, packedObjectMap2, sharedValues = options2.sharedValues;
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
      referenceMap2 = encoder.structuredClone ? /* @__PURE__ */ new Map() : null;
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
      if (options2.pack) {
        let packedValues2 = /* @__PURE__ */ new Map();
        packedValues2.values = [];
        packedValues2.encoder = encoder;
        packedValues2.maxValues = options2.maxPrivatePackedValues || (sharedPackedObjectMap2 ? 16 : Infinity);
        packedValues2.objectMap = sharedPackedObjectMap2 || false;
        packedValues2.samplingPackedValues = samplingPackedValues;
        findRepetitiveStrings(value, packedValues2);
        if (packedValues2.values.length > 0) {
          target[position2++] = 216;
          target[position2++] = 51;
          writeArrayHeader(4);
          let valuesArray = packedValues2.values;
          encode2(valuesArray);
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
        encode2(value);
        if (bundledStrings2) {
          writeBundles(start, encode2);
        }
        encoder.offset = position2;
        if (referenceMap2 && referenceMap2.idsToInsert) {
          position2 += referenceMap2.idsToInsert.length * 2;
          if (position2 > safeEnd)
            makeRoom(position2);
          encoder.offset = position2;
          let serialized = insertIds(target.subarray(start, position2), referenceMap2.idsToInsert);
          referenceMap2 = null;
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
      return (options3) => {
        let threshold = options3 && options3.threshold || 4;
        let position3 = this.pack ? options3.maxPrivatePackedValues || 16 : 0;
        if (!sharedValues)
          sharedValues = this.sharedValues = [];
        for (let [key, status] of samplingPackedValues) {
          if (status.count > threshold) {
            sharedPackedObjectMap2[key] = position3++;
            sharedValues.push(key);
            hasSharedUpdate = true;
          }
        }
        while (this.saveShared && this.updateSharedData() === false) {
        }
        samplingPackedValues = null;
      };
    };
    const encode2 = (value) => {
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
                encode2(15 - packedPosition >> 1);
              else
                encode2(packedPosition - 16 >> 1);
            }
            return;
          } else if (samplingPackedValues && !options2.pack) {
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
              writeBundles(start, encode2);
            }
            bundledStrings2 = ["", ""];
            bundledStrings2.size = 0;
            bundledStrings2.position = extStart;
          }
          let twoByte = hasNonLatin.test(value);
          bundledStrings2[twoByte ? 0 : 1] += value;
          target[position2++] = twoByte ? 206 : 207;
          encode2(strLength);
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
            if (useFloat32 < 4 || (xShifted = value * mult10[(target[position2] & 127) << 1 | target[position2 + 1] >> 7]) >> 0 === xShifted) {
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
          if (referenceMap2) {
            let referee = referenceMap2.get(value);
            if (referee) {
              target[position2++] = 216;
              target[position2++] = 29;
              target[position2++] = 25;
              if (!referee.references) {
                let idsToInsert = referenceMap2.idsToInsert || (referenceMap2.idsToInsert = []);
                referee.references = [];
                idsToInsert.push(referee);
              }
              referee.references.push(position2 - start);
              position2 += 2;
              return;
            } else
              referenceMap2.set(value, { offset: position2 - start });
          }
          let constructor = value.constructor;
          if (constructor === Object) {
            writeObject(value, true);
          } else if (constructor === Array) {
            length = value.length;
            if (length < 24) {
              target[position2++] = 128 | length;
            } else {
              writeArrayHeader(length);
            }
            for (let i = 0; i < length; i++) {
              encode2(value[i]);
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
                encode2(encoder.encodeKey(key));
                encode2(entryValue);
              }
            } else {
              for (let [key, entryValue] of value) {
                encode2(key);
                encode2(entryValue);
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
                extension.encode.call(this, value, encode2, makeRoom);
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
                encode2(entry);
              }
              target[position2++] = 255;
              return;
            }
            if (value[Symbol.asyncIterator] || isBlob(value)) {
              let error = new Error("Iterable/blob should be serialized as iterator");
              error.iteratorNotHandled = true;
              throw error;
            }
            writeObject(value, !value.hasOwnProperty);
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
            throw new RangeError(value + " was too large to fit in CBOR 64-bit integer format, set largeBigIntToFloat to convert to float-64");
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
          encode2(encodeKey(keys[i]));
          encode2(vals[i]);
        }
      } else {
        for (let i = 0; i < length; i++) {
          encode2(keys[i]);
          encode2(vals[i]);
        }
      }
    } : (object, safePrototype) => {
      target[position2++] = 185;
      let objectOffset = position2 - start;
      position2 += 2;
      let size = 0;
      if (encoder.keyMap) {
        for (let key in object)
          if (safePrototype || object.hasOwnProperty(key)) {
            encode2(encoder.encodeKey(key));
            encode2(object[key]);
            size++;
          }
      } else {
        for (let key in object)
          if (safePrototype || object.hasOwnProperty(key)) {
            encode2(key);
            encode2(object[key]);
            size++;
          }
      }
      target[objectOffset++ + start] = size >> 8;
      target[objectOffset + start] = size & 255;
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
          encode2(57344 + recordId);
          encode2(keys);
          if (safePrototype === null)
            return;
          for (let key in object)
            if (safePrototype || object.hasOwnProperty(key))
              encode2(object[key]);
          return;
        }
      }
      if (length < 24) {
        target[position2++] = 128 | length;
      } else {
        writeArrayHeader(length);
      }
      if (safePrototype === null)
        return;
      for (let key in object)
        if (safePrototype || object.hasOwnProperty(key))
          encode2(object[key]);
    };
    const makeRoom = (end) => {
      let newSize;
      if (end > 16777216) {
        if (end - start > MAX_BUFFER_SIZE)
          throw new Error("Encoded buffer would be larger than maximum buffer size");
        newSize = Math.min(MAX_BUFFER_SIZE, Math.round(Math.max((end - start) * (end > 67108864 ? 1.25 : 2), 4194304) / 4096) * 4096);
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
    this.encodeAsIterable = function(value, options3) {
      return startEncoding(value, options3, encodeObjectAsIterable);
    };
    this.encodeAsAsyncIterable = function(value, options3) {
      return startEncoding(value, options3, encodeObjectAsAsyncIterable);
    };
    function* encodeObjectAsIterable(object, iterateProperties, finalIterable) {
      let constructor = object.constructor;
      if (constructor === Object) {
        let useRecords = encoder.useRecords !== false;
        if (useRecords)
          writeObject(object, null);
        else
          writeEntityLength(Object.keys(object).length, 160);
        for (let key in object) {
          let value = object[key];
          if (!useRecords)
            encode2(key);
          if (value && typeof value === "object") {
            if (iterateProperties[key])
              yield* encodeObjectAsIterable(value, iterateProperties[key]);
            else
              yield* tryEncode(value, iterateProperties, key);
          } else
            encode2(value);
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
          } else
            encode2(value);
        }
      } else if (object[Symbol.iterator]) {
        target[position2++] = 159;
        for (let value of object) {
          if (value && (typeof value === "object" || position2 - start > chunkThreshold)) {
            if (iterateProperties.element)
              yield* encodeObjectAsIterable(value, iterateProperties.element);
            else
              yield* tryEncode(value, iterateProperties, "element");
          } else
            encode2(value);
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
        encode2(object);
      }
      if (finalIterable && position2 > start)
        yield target.subarray(start, position2);
      else if (position2 - start > chunkThreshold) {
        yield target.subarray(start, position2);
        restartEncoding();
      }
    }
    function* tryEncode(value, iterateProperties, key) {
      let restart = position2 - start;
      try {
        encode2(value);
        if (position2 - start > chunkThreshold) {
          yield target.subarray(start, position2);
          restartEncoding();
        }
      } catch (error) {
        if (error.iteratorNotHandled) {
          iterateProperties[key] = {};
          position2 = start + restart;
          yield* encodeObjectAsIterable.call(this, value, iterateProperties[key]);
        } else
          throw error;
      }
    }
    function restartEncoding() {
      chunkThreshold = continuedChunkThreshold;
      encoder.encode(null, THROW_ON_ITERABLE);
    }
    function startEncoding(value, options3, encodeIterable) {
      if (options3 && options3.chunkThreshold)
        chunkThreshold = continuedChunkThreshold = options3.chunkThreshold;
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
function findRepetitiveStrings(value, packedValues2) {
  switch (typeof value) {
    case "string":
      if (value.length > 3) {
        if (packedValues2.objectMap[value] > -1 || packedValues2.values.length >= packedValues2.maxValues)
          return;
        let packedStatus = packedValues2.get(value);
        if (packedStatus) {
          if (++packedStatus.count == 2) {
            packedValues2.values.push(value);
          }
        } else {
          packedValues2.set(value, {
            count: 1
          });
          if (packedValues2.samplingPackedValues) {
            let status = packedValues2.samplingPackedValues.get(value);
            if (status)
              status.count++;
            else
              packedValues2.samplingPackedValues.set(value, {
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
            findRepetitiveStrings(value[i], packedValues2);
          }
        } else {
          let includeKeys = !packedValues2.encoder.useRecords;
          for (var key in value) {
            if (value.hasOwnProperty(key)) {
              if (includeKeys)
                findRepetitiveStrings(key, packedValues2);
              findRepetitiveStrings(value[key], packedValues2);
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
    tag: 1,
    encode(date, encode2) {
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
    tag: 258,
    encode(set, encode2) {
      let array = Array.from(set);
      encode2(array);
    }
  },
  {
    tag: 27,
    encode(error, encode2) {
      encode2([error.name, error.message]);
    }
  },
  {
    tag: 27,
    encode(regex, encode2) {
      encode2(["RegExp", regex.source, regex.flags]);
    }
  },
  {
    getTag(tag) {
      return tag.tag;
    },
    encode(tag, encode2) {
      encode2(tag.value);
    }
  },
  {
    encode(arrayBuffer, encode2, makeRoom) {
      writeBuffer(arrayBuffer, makeRoom);
    }
  },
  {
    getTag(typedArray) {
      if (typedArray.constructor === Uint8Array) {
        if (this.tagUint8Array || hasNodeBuffer && this.tagUint8Array !== false)
          return 64;
      }
    },
    encode(typedArray, encode2, makeRoom) {
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
    encode(sharedData, encode2) {
      let packedValues2 = sharedData.packedValues || [];
      let sharedStructures = sharedData.structures || [];
      if (packedValues2.values.length > 0) {
        target[position2++] = 216;
        target[position2++] = 51;
        writeArrayHeader(4);
        let valuesArray = packedValues2.values;
        encode2(valuesArray);
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
        encode2(definitions);
      } else
        encode2(new Tag(sharedData.version, 1399353956));
    }
  }
];
function typedArrayEncoder(tag, size) {
  if (!isLittleEndianMachine2 && size > 1)
    tag -= 4;
  return {
    tag,
    encode: function writeExtBuffer(typedArray, encode2) {
      let length = typedArray.byteLength;
      let offset = typedArray.byteOffset || 0;
      let buffer = typedArray.buffer || typedArray;
      encode2(hasNodeBuffer ? Buffer2.from(buffer, offset, length) : new Uint8Array(buffer, offset, length));
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
    for (let position3 of referee.references) {
      serialized[position3++] = id >> 8;
      serialized[position3] = id & 255;
    }
  }
  while (nextId = idsToInsert.pop()) {
    let offset = nextId.offset;
    serialized.copyWithin(offset + distanceToMove, offset, lastEnd);
    distanceToMove -= 2;
    let position3 = offset + distanceToMove;
    serialized[position3++] = 216;
    serialized[position3++] = 28;
    lastEnd = offset;
  }
  return serialized;
}
function writeBundles(start, encode2) {
  targetView.setUint32(bundledStrings2.position + start, position2 - bundledStrings2.position - start + 1);
  let writeStrings = bundledStrings2;
  bundledStrings2 = null;
  encode2(writeStrings[0]);
  encode2(writeStrings[1]);
}
function addExtension2(extension) {
  if (extension.Class) {
    if (!extension.encode)
      throw new Error("Extension has no encode function");
    extensionClasses.unshift(extension.Class);
    extensions.unshift(extension);
  }
  addExtension(extension);
}
var defaultEncoder = new Encoder({ useRecords: false });
var encode = defaultEncoder.encode;
var encodeAsIterable = defaultEncoder.encodeAsIterable;
var encodeAsAsyncIterable = defaultEncoder.encodeAsAsyncIterable;
var REUSE_BUFFER_MODE = 512;
var RESET_BUFFER_MODE = 1024;
var THROW_ON_ITERABLE = 2048;

// codec/cbor.ts
var CBORCodec = class {
  constructor(debug2 = false, extensions2) {
    this.debug = debug2;
    if (extensions2) {
      extensions2.forEach(addExtension2);
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
  constructor(w, debug2 = false) {
    this.w = w;
    this.debug = debug2;
  }
  async encode(v) {
    if (this.debug) {
      console.log("<<", v);
    }
    let buf = encode(v);
    let nwritten = 0;
    while (nwritten < buf.length) {
      nwritten += await this.w.write(buf.subarray(nwritten));
    }
  }
};
var CBORDecoder = class {
  constructor(r, debug2 = false) {
    this.r = r;
    this.debug = debug2;
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
    let v = decode(buf);
    if (this.debug) {
      console.log(">>", v);
    }
    return Promise.resolve(v);
  }
};

// buffer.ts
function copy(src2, dst, off = 0) {
  off = Math.max(0, Math.min(off, dst.byteLength));
  const dstBytesAvailable = dst.byteLength - off;
  if (src2.byteLength > dstBytesAvailable) {
    src2 = src2.subarray(0, dstBytesAvailable);
  }
  dst.set(src2, off);
  return src2.byteLength;
}
var MIN_READ = 32 * 1024;
var MAX_SIZE = 2 ** 32 - 2;
var Buffer3 = class {
  constructor(ab) {
    this._buf = ab === void 0 ? new Uint8Array(0) : new Uint8Array(ab);
    this._off = 0;
  }
  bytes(options2 = { copy: true }) {
    if (options2.copy === false)
      return this._buf.subarray(this._off);
    return this._buf.slice(this._off);
  }
  empty() {
    return this._buf.byteLength <= this._off;
  }
  get length() {
    return this._buf.byteLength - this._off;
  }
  get capacity() {
    return this._buf.buffer.byteLength;
  }
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
  read(p) {
    const rr = this.readSync(p);
    return Promise.resolve(rr);
  }
  writeSync(p) {
    const m = this._grow(p.byteLength);
    return copy(p, this._buf, m);
  }
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
  grow(n) {
    if (n < 0) {
      throw Error("Buffer.grow: negative count");
    }
    const m = this._grow(n);
    this._reslice(m);
  }
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

// codec/frame.ts
var FrameCodec = class {
  constructor(codec) {
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
    this.w = w;
    this.codec = codec;
  }
  async encode(v) {
    const data = new Buffer3();
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

// rpc/client.ts
var Client = class {
  constructor(session, codec) {
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
      const resp = new Response(ch, framer);
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
      console.warn(e, selector, args);
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

// rpc/handler.ts
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

// rpc/responder.ts
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

// rpc/mod.ts
var Call = class {
  constructor(selector, channel, decoder2) {
    this.selector = selector;
    this.channel = channel;
    this.decoder = decoder2;
  }
  receive() {
    return this.decoder.decode();
  }
};
var ResponseHeader = class {
  constructor() {
    this.E = void 0;
    this.C = false;
  }
};
var Response = class {
  constructor(channel, codec) {
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

// peer/peer.ts
var Peer = class {
  constructor(session, codec) {
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
  virtualize() {
    return VirtualCaller(this.caller);
  }
};

// mux/codec/message.ts
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

// mux/codec/encoder.ts
var Encoder2 = class {
  constructor(w) {
    this.w = w;
  }
  async encode(m) {
    if (debug.messages) {
      console.log("<<ENC", m);
    }
    const buf = Marshal(m);
    if (debug.bytes) {
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

// mux/util.ts
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

// mux/codec/decoder.ts
var Decoder2 = class {
  constructor(r) {
    this.r = r;
  }
  async decode() {
    const packet = await readPacket(this.r);
    if (packet === null) {
      return Promise.resolve(null);
    }
    if (debug.bytes) {
      console.log(">>DEC", packet);
    }
    const msg = Unmarshal(packet);
    if (debug.messages) {
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
    return Promise.reject("unexpected EOF");
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
        return Promise.reject("unexpected EOF");
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

// mux/codec/mod.ts
var debug = {
  messages: false,
  bytes: false
};

// mux/session/session.ts
var minPacketLength = 9;
var maxPacketLength = Number.MAX_VALUE;
var Session = class {
  constructor(conn) {
    this.conn = conn;
    this.enc = new Encoder2(conn);
    this.dec = new Decoder2(conn);
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
          throw `invalid channel (${cmsg.channelID}) on op ${cmsg.ID}`;
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

// mux/session/channel.ts
var channelMaxPacket = 1 << 24;
var channelWindowSize = 64 * channelMaxPacket;
var Channel = class {
  constructor(sess) {
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
  ident() {
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

// transport/websocket.ts
var websocket_exports = {};
__export(websocket_exports, {
  Conn: () => Conn,
  connect: () => connect
});
function connect(addr, onclose) {
  return new Promise((resolve) => {
    const socket = new WebSocket(addr);
    socket.onopen = () => resolve(new Conn(socket));
    if (onclose)
      socket.onclose = onclose;
  });
}
var Conn = class {
  constructor(ws) {
    this.isClosed = false;
    this.waiters = [];
    this.chunks = [];
    this.ws = ws;
    this.ws.binaryType = "arraybuffer";
    this.ws.onmessage = (event) => {
      const chunk = new Uint8Array(event.data);
      this.chunks.push(chunk);
      if (this.waiters.length > 0) {
        const waiter = this.waiters.shift();
        if (waiter)
          waiter();
      }
    };
    const onclose = this.ws.onclose;
    this.ws.onclose = (e) => {
      if (onclose)
        onclose.bind(this.ws)(e);
      this.close();
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
            resolve(null);
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
    this.ws.send(p);
    return Promise.resolve(p.byteLength);
  }
  close() {
    if (this.isClosed)
      return;
    this.isClosed = true;
    this.waiters.forEach((waiter) => waiter());
    this.ws.close();
  }
};

// peer/mod.ts
var options = {
  transport: websocket_exports
};
async function connect2(addr, codec) {
  const conn = await options.transport.connect(addr);
  return open(conn, codec);
}
function open(conn, codec, handlers) {
  const sess = new Session(conn);
  const p = new Peer(sess, codec);
  if (handlers) {
    for (const name in handlers) {
      p.handle(name, HandlerFunc(handlers[name]));
    }
    p.respond();
  }
  return p;
}

// fn/handler.ts
function handlerFrom(v) {
  if (v instanceof Function) {
    return handlerFromFunc(v);
  }
  return handlerFromObj(v);
}
function handlerFromObj(obj) {
  const handler = new RespondMux();
  for (const prop of getObjectProps(obj)) {
    if (["constructor", "respondRPC"].includes(prop) || prop.startsWith("_")) {
      continue;
    }
    if (obj[prop] instanceof Function) {
      let h = handlerFromFunc(obj[prop], obj);
      if (obj[`_${prop}RPC`] instanceof Function) {
        h = { "respondRPC": obj[`_${prop}RPC`].bind(obj) };
      }
      handler.handle(prop, h);
    } else if (obj[prop] && obj[prop]["respondRPC"]) {
      handler.handle(prop, obj[prop]);
    }
  }
  if (obj["respondRPC"]) {
    handler.handle("/", obj);
  }
  return handler;
}
function handlerFromFunc(fn, thisArg) {
  return HandlerFunc(async (r, c) => {
    try {
      const ret = fn.apply(thisArg, await c.receive());
      if (ret instanceof Promise) {
        const v = await ret;
        r.return(v);
      } else {
        r.return(ret);
      }
    } catch (e) {
      if (typeof e === "string") {
        r.return(new Error(e));
        return;
      }
      r.return(e);
    }
  });
}
function getObjectProps(v) {
  const names = /* @__PURE__ */ new Set();
  Object.getOwnPropertyNames(v).forEach((n) => names.add(n));
  for (let p = v; p != null; p = Object.getPrototypeOf(p)) {
    if (p.constructor.name !== "Object") {
      Object.getOwnPropertyNames(p).forEach((n) => names.add(n));
    }
  }
  return [...names];
}

// fn/proxy.ts
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

// io.ts
var EOF = null;
async function copy2(dst, src2) {
  const buf = new Uint8Array(32 * 1024);
  let written = 0;
  while (true) {
    const n = await src2.read(buf);
    if (n === EOF) {
      break;
    }
    written += await dst.write(buf.subarray(0, n));
  }
  return written;
}

// transport/iframe.ts
var frames = {};
function frameElementID(w) {
  return w.frameElement ? w.frameElement.id : "";
}
if (globalThis.window) {
  window.addEventListener("message", (event) => {
    if (!event.source)
      return;
    const frameID = frameElementID(event.source);
    if (!frames[frameID]) {
      const event2 = new CustomEvent("connection", { detail: frameID });
      if (!window.dispatchEvent(event2)) {
        return;
      }
      if (!frames[frameID]) {
        console.warn("incoming message with no connection for frame ID in window:", frameID, window.location);
        return;
      }
    }
    const conn = frames[frameID];
    const chunk = new Uint8Array(event.data);
    conn.chunks.push(chunk);
    if (conn.waiters.length > 0) {
      const waiter = conn.waiters.shift();
      if (waiter)
        waiter();
    }
  });
}
var Conn2 = class {
  constructor(frame) {
    this.isClosed = false;
    this.waiters = [];
    this.chunks = [];
    if (frame && frame.contentWindow) {
      this.frame = frame.contentWindow;
      frames[frame.id] = this;
    } else {
      this.frame = window.parent;
      frames[frameElementID(window.parent)] = this;
    }
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
            resolve(null);
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
    this.frame.postMessage(p.buffer, "*");
    return Promise.resolve(p.byteLength);
  }
  close() {
    if (this.isClosed)
      return;
    this.isClosed = true;
    this.waiters.forEach((waiter) => waiter());
  }
};

// transport/worker.ts
var Conn3 = class {
  constructor(worker) {
    this.isClosed = false;
    this.waiters = [];
    this.chunks = [];
    this.worker = worker;
    this.worker.onmessage = (event) => {
      if (!event.data.duplex)
        return;
      const chunk = new Uint8Array(event.data.duplex);
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
            resolve(null);
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
    this.worker.postMessage({ duplex: p.buffer });
    return Promise.resolve(p.byteLength);
  }
  close() {
    if (this.isClosed)
      return;
    this.isClosed = true;
    this.waiters.forEach((waiter) => waiter());
  }
};
export {
  Buffer3 as Buffer,
  CBORCodec,
  CBORDecoder,
  CBOREncoder,
  Call,
  Channel,
  Client,
  EOF,
  FrameCodec,
  Conn2 as FrameConn,
  FrameDecoder,
  FrameEncoder,
  HandlerFunc,
  JSONCodec,
  JSONDecoder,
  JSONEncoder,
  NotFoundHandler,
  Peer,
  Respond,
  RespondMux,
  Response,
  ResponseHeader,
  Session,
  VirtualCaller,
  Conn3 as WorkerConn,
  channelMaxPacket,
  channelWindowSize,
  connect2 as connect,
  copy2 as copy,
  handlerFrom,
  maxPacketLength,
  methodProxy,
  minPacketLength,
  open,
  options
};
//# sourceURL=duplex.js