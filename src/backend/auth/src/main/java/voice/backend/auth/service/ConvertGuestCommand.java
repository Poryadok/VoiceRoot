package voice.backend.auth.service;

public record ConvertGuestCommand(String email, String phone, String password) {}
