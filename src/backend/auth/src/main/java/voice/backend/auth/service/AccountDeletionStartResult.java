package voice.backend.auth.service;

import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountDeletionOperation;

/** The account transition and its durable completion operation, observed from one start attempt. */
public record AccountDeletionStartResult(Account account, AccountDeletionOperation operation) {}
