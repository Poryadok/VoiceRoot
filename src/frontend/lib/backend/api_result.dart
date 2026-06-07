import 'gateway_api_error.dart';

/// Maps [GatewayApiError] into legacy client failure shapes.
mixin GatewayApiResultMapper {
  static String failureMessage(GatewayApiError error) => error.message;

  static String? failureCode(GatewayApiError error) => error.errorCode;

  static int? failureStatus(GatewayApiError error) =>
      error.statusCode == 0 ? null : error.statusCode;
}
