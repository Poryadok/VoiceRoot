import '../gen/voice/subscription/v1/subscription.pb.dart' as sub_pb;
import 'api_result.dart';
import 'gateway_http.dart';
import 'proto_mappers.dart';

const String kSubscriptionMissingBaseUrlDetail = 'missing base URL';

sealed class SubscriptionApiResult<T> {
  const SubscriptionApiResult();
}

final class SubscriptionApiOk<T> extends SubscriptionApiResult<T> {
  const SubscriptionApiOk(this.data);
  final T data;
}

final class SubscriptionApiFailure extends SubscriptionApiResult<Never> {
  const SubscriptionApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class VoiceSubscription {
  const VoiceSubscription({
    required this.id,
    required this.accountId,
    required this.plan,
    required this.billingPeriod,
    required this.status,
    this.provider,
    this.providerSubscriptionId,
    this.currentPeriodEnd,
  });

  final String id;
  final String accountId;
  final String plan;
  final String billingPeriod;
  final String status;
  final String? provider;
  final String? providerSubscriptionId;
  final DateTime? currentPeriodEnd;

  bool get isPremium => plan == 'premium' && status != 'cancelled';
}

class VoiceCheckoutSession {
  const VoiceCheckoutSession({
    required this.checkoutUrl,
    required this.sessionId,
  });

  final String checkoutUrl;
  final String sessionId;
}

class VoiceSubscriptionLimits {
  const VoiceSubscriptionLimits({required this.limitsJson});

  final String limitsJson;
}

class DowngradeProfilesResult {
  const DowngradeProfilesResult({required this.keptProfileIds});

  final List<String> keptProfileIds;
}

/// HTTP client for Subscription routes via API Gateway (`/api/v1/subscription/**`).
class VoiceSubscriptionClient {
  VoiceSubscriptionClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<SubscriptionApiResult<VoiceSubscription>> getSubscription({
    required String authorization,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/subscription/me'),
      authorization: authorization,
      createEmpty: sub_pb.GetSubscriptionResponse.create,
    );
    return _map(
      result,
      (data) => voiceSubscriptionFromProto(
        data.hasSubscription() ? data.subscription : sub_pb.Subscription(),
      ),
    );
  }

  Future<SubscriptionApiResult<VoiceCheckoutSession>> createCheckoutSession({
    required String authorization,
    required String plan,
    required String billingPeriod,
    required String successUrl,
    required String cancelUrl,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/subscription/checkout'),
      authorization: authorization,
      body: createCheckoutSessionRequestToProto(
        plan: plan,
        billingPeriod: billingPeriod,
        successUrl: successUrl,
        cancelUrl: cancelUrl,
      ),
      createEmpty: sub_pb.CreateCheckoutSessionResponse.create,
    );
    return _map(
      result,
      (data) => voiceCheckoutSessionFromProto(
        data.hasCheckoutResponse()
            ? data.checkoutResponse
            : sub_pb.CheckoutResponse(),
      ),
    );
  }

  Future<SubscriptionApiResult<VoiceSubscription>> cancelSubscription({
    required String authorization,
    required String subscriptionId,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/subscription/cancel'),
      authorization: authorization,
      body: sub_pb.CancelSubscriptionRequest(subscriptionId: subscriptionId),
      createEmpty: sub_pb.CancelSubscriptionResponse.create,
    );
    return _map(
      result,
      (data) => voiceSubscriptionFromProto(
        data.hasSubscription() ? data.subscription : sub_pb.Subscription(),
      ),
    );
  }

  Future<SubscriptionApiResult<VoiceSubscriptionLimits>> getLimits({
    required String authorization,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/subscription/limits'),
      authorization: authorization,
      createEmpty: sub_pb.GetLimitsResponse.create,
    );
    return _map(
      result,
      (data) => voiceSubscriptionLimitsFromProto(
        data.hasLimits() ? data.limits : sub_pb.Limits(),
      ),
    );
  }

  Future<SubscriptionApiResult<DowngradeProfilesResult>> submitDowngradeProfiles({
    required String authorization,
    required List<String> keptProfileIds,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/subscription/downgrade/profiles'),
      authorization: authorization,
      body: {'kept_profile_ids': keptProfileIds},
    );
    return _mapJson(
      result,
      (data) => DowngradeProfilesResult(
        keptProfileIds: (data['kept_profile_ids'] as List<dynamic>?)
                ?.map((e) => '$e')
                .toList(growable: false) ??
            keptProfileIds,
      ),
    );
  }

  SubscriptionApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => SubscriptionApiOk(parse(data)),
      GatewayHttpFailure(:final error) => SubscriptionApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  SubscriptionApiResult<T> _mapJson<T>(
    GatewayHttpResult<Map<String, dynamic>> result,
    T Function(Map<String, dynamic> data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => SubscriptionApiOk(parse(data)),
      GatewayHttpFailure(:final error) => SubscriptionApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
