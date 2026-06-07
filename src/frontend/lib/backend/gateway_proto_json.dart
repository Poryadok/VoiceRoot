import 'dart:convert';

import 'package:fixnum/fixnum.dart';
import 'package:protobuf/protobuf.dart';

/// Decode Gateway gRPC-transcoded JSON ([UseProtoNames] snake_case).
T decodeGatewayProto<T extends GeneratedMessage>(
  T Function() createEmpty,
  String body,
) {
  if (body.isEmpty) {
    return createEmpty();
  }
  final decoded = jsonDecode(body);
  final msg = createEmpty();
  if (decoded is Map<String, dynamic>) {
    msg.mergeFromProto3Json(decoded, ignoreUnknownFields: true);
  }
  return msg;
}

/// Encode request body for Gateway gRPC transcoding (proto field names, snake_case).
String encodeGatewayProto(GeneratedMessage message) {
  return jsonEncode(_writeGatewayProtoJson(message));
}

/// Decode when the JSON root is not the message itself (e.g. nested `message`).
T decodeGatewayProtoField<T extends GeneratedMessage>(
  T Function() createEmpty,
  Map<String, dynamic> body,
  String field,
) {
  final raw = body[field];
  if (raw is! Map<String, dynamic>) {
    return createEmpty();
  }
  final msg = createEmpty();
  msg.mergeFromProto3Json(raw, ignoreUnknownFields: true);
  return msg;
}

Map<String, dynamic> _writeGatewayProtoJson(GeneratedMessage message) {
  final result = <String, dynamic>{};
  for (final field in message.info_.sortedByTag) {
    if (!message.hasField(field.tagNumber)) continue;
    final value = message.getField(field.tagNumber);
    if (value == null) continue;
    final encoded = _encodeGatewayFieldValue(value, field);
    if (encoded == null) continue;
    result[field.protoName] = encoded;
  }
  return result;
}

Object? _encodeGatewayFieldValue(Object? value, FieldInfo field) {
  if (value == null) return null;

  if (field.isMapField) {
    final mapField = field as MapFieldInfo;
    return (value as PbMap).map((key, entryValue) {
      return MapEntry(
        _encodeMapKey(key, mapField.keyFieldType),
        _encodeScalarOrMessage(entryValue, mapField.valueFieldType, field),
      );
    });
  }

  if (field.isRepeated) {
    final list = value as List;
    if (list.isEmpty) return null;
    return list
        .map((element) => _encodeScalarOrMessage(element, field.type, field))
        .toList(growable: false);
  }

  return _encodeScalarOrMessage(value, field.type, field);
}

Object? _encodeScalarOrMessage(
  Object? value,
  int fieldType,
  FieldInfo field,
) {
  if (value == null) return null;
  if (PbFieldType.isGroupOrMessage(fieldType)) {
    return _writeGatewayProtoJson(value as GeneratedMessage);
  }
  if (PbFieldType.isEnum(fieldType)) {
    return (value as ProtobufEnum).name;
  }
  return _encodeScalar(value, fieldType);
}

Object? _encodeScalar(Object? value, int fieldType) {
  final baseType = PbFieldType.baseType(fieldType);
  switch (baseType) {
    case PbFieldType.BOOL_BIT:
    case PbFieldType.STRING_BIT:
    case PbFieldType.INT32_BIT:
    case PbFieldType.SINT32_BIT:
    case PbFieldType.UINT32_BIT:
    case PbFieldType.FIXED32_BIT:
    case PbFieldType.SFIXED32_BIT:
    case PbFieldType.FLOAT_BIT:
    case PbFieldType.DOUBLE_BIT:
      return value;
    case PbFieldType.INT64_BIT:
    case PbFieldType.SINT64_BIT:
    case PbFieldType.SFIXED64_BIT:
    case PbFieldType.FIXED64_BIT:
    case PbFieldType.UINT64_BIT:
      return value.toString();
    case PbFieldType.BYTES_BIT:
      return base64Encode(value as List<int>);
    default:
      return value;
  }
}

String _encodeMapKey(Object key, int keyFieldType) {
  final baseType = PbFieldType.baseType(keyFieldType);
  switch (baseType) {
    case PbFieldType.BOOL_BIT:
      return key == true ? 'true' : 'false';
    case PbFieldType.STRING_BIT:
      return key as String;
    case PbFieldType.UINT64_BIT:
      return (key as Int64).toStringUnsigned();
    default:
      return key.toString();
  }
}
