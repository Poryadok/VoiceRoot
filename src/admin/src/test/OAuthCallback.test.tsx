import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { OAuthCallback } from "../oauth/OAuthCallback";
import * as oauthApi from "../oauth/api";
import { setOAuthState, setPkceVerifier } from "../oauth/session";

vi.mock("../oauth/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../oauth/api")>();
  return {
    ...actual,
    exchangeAuthorizationCode: vi.fn(),
  };
});

describe("OAuthCallback", () => {
  const replaceSpy = vi.fn();

  beforeEach(() => {
    sessionStorage.clear();
    replaceSpy.mockReset();
    vi.mocked(oauthApi.exchangeAuthorizationCode).mockReset();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        ...window.location,
        origin: "https://admin.test",
        search: "",
        replace: replaceSpy,
      },
    });
  });

  it("shows oauth error from query string", async () => {
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        ...window.location,
        origin: "https://admin.test",
        search: "?error=access_denied",
        replace: replaceSpy,
      },
    });

    render(<OAuthCallback />);

    expect(await screen.findByRole("alert")).toHaveTextContent("access_denied");
    expect(oauthApi.exchangeAuthorizationCode).not.toHaveBeenCalled();
  });

  it("shows missing_code when authorization code absent", async () => {
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        ...window.location,
        origin: "https://admin.test",
        search: "?state=abc",
        replace: replaceSpy,
      },
    });

    render(<OAuthCallback />);

    expect(await screen.findByRole("alert")).toHaveTextContent("missing_code");
  });

  it("shows invalid_state when stored state mismatches", async () => {
    setOAuthState("expected-state");
    setPkceVerifier("verifier");
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        ...window.location,
        origin: "https://admin.test",
        search: "?code=auth-code&state=wrong-state",
        replace: replaceSpy,
      },
    });

    render(<OAuthCallback />);

    expect(await screen.findByRole("alert")).toHaveTextContent("invalid_state");
  });

  it("exchanges code and redirects home on success", async () => {
    setOAuthState("state-1");
    setPkceVerifier("pkce-verifier");
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        ...window.location,
        origin: "https://admin.test",
        search: "?code=auth-code&state=state-1",
        replace: replaceSpy,
      },
    });
    vi.mocked(oauthApi.exchangeAuthorizationCode).mockResolvedValue({
      access_token: "staff-jwt",
      token_type: "Bearer",
      expires_in: 3600,
    });

    render(<OAuthCallback />);

    await waitFor(() => {
      expect(oauthApi.exchangeAuthorizationCode).toHaveBeenCalledWith({
        code: "auth-code",
        redirectUri: "https://admin.test/callback",
        codeVerifier: "pkce-verifier",
      });
    });
    await waitFor(() => {
      expect(replaceSpy).toHaveBeenCalledWith("/");
    });
    expect(sessionStorage.getItem("voice_admin_access_token")).toBe("staff-jwt");
  });

  it("shows token exchange failure message", async () => {
    setOAuthState("state-1");
    setPkceVerifier("pkce-verifier");
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        ...window.location,
        origin: "https://admin.test",
        search: "?code=auth-code&state=state-1",
        replace: replaceSpy,
      },
    });
    vi.mocked(oauthApi.exchangeAuthorizationCode).mockRejectedValue(
      new Error("token_exchange_failed"),
    );

    render(<OAuthCallback />);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "token_exchange_failed",
    );
  });
});
