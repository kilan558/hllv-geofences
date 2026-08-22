export class ConfigurationError extends Error {
  constructor(message) {
    super(message);
    this.name = "ConfigurationError";
    this.code = "CONFIGURATION_ERROR";
  }
}

export class WorkerActionError extends Error {
  constructor(message, code = "WORKER_ACTION_FAILED", publicMessage = message) {
    super(message);
    this.name = "WorkerActionError";
    this.code = code;
    this.publicMessage = publicMessage;
  }
}
