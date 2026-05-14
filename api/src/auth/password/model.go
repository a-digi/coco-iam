package password

// ContextBagKey under which the Service is registered in the DI bag so
// handlers can resolve it. Matches the convention used by other domain
// services (activation, mail, etc.).
const ContextBagKeyService = "password.Service"
