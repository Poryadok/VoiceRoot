import 'dart:convert';

import '../gen/voice/moderation/v1/moderation.pb.dart' as moderation_pb;
import 'api_result.dart';
import 'gateway_http.dart';

const String kModerationMissingBaseUrlDetail = 'missing base URL';

sealed class ModerationApiResult<T> {
  const ModerationApiResult();
}

final class ModerationApiOk<T> extends ModerationApiResult<T> {
  const ModerationApiOk(this.data);
  final T data;
}

final class ModerationApiFailure extends ModerationApiResult<Never> {
  const ModerationApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class ReportSubmission {
  const ReportSubmission({required this.reportId});

  final String reportId;
}

/// HTTP client for Moderation routes via API Gateway (`/api/v1/moderation/**`).
class VoiceModerationClient {
  VoiceModerationClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<ModerationApiResult<ReportSubmission>> createReport({
    required String authorization,
    required String targetType,
    required String targetId,
    required String category,
    String? description,
    Map<String, Object?> evidence = const {},
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/moderation/reports'),
      authorization: authorization,
      body: moderation_pb.CreateReportRequest(
        targetType: targetType,
        targetId: targetId,
        category: category,
        description: description,
        evidenceJson: jsonEncode(evidence),
      ),
      createEmpty: moderation_pb.CreateReportResponse.create,
    );
    return _map(
      result,
      (data) => ReportSubmission(
        reportId: data.hasReport() ? data.report.id : '',
      ),
    );
  }

  ModerationApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(moderation_pb.CreateReportResponse data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => ModerationApiOk(parse(data)),
      GatewayHttpFailure(:final error) => ModerationApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
