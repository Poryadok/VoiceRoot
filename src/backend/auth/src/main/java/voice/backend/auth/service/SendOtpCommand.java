package voice.backend.auth.service;

public record SendOtpCommand(String email, String phone, String otpType, String accessToken) {}
