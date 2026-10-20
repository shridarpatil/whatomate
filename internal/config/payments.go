package config

// PaymentsConfig contains server-only credentials. New invoices use Mercado
// Pago unless EFIContingency is explicitly enabled. Existing invoices keep
// their provider, including when checkout is disabled.
type PaymentsConfig struct {
	Enabled           bool   `koanf:"enabled"`
	EFIContingency    bool   `koanf:"efi_contingency"`
	MPAccessToken     string `koanf:"mp_access_token"`
	MPWebhookSecret   string `koanf:"mp_webhook_secret"`
	MPNotificationURL string `koanf:"mp_notification_url"`
	EFIClientID       string `koanf:"efi_client_id"`
	EFIClientSecret   string `koanf:"efi_client_secret"`
	EFIPixKey         string `koanf:"efi_pix_key"`
	EFICertificate    string `koanf:"efi_certificate"`
	EFIPrivateKey     string `koanf:"efi_private_key"`
	EFISandbox        bool   `koanf:"efi_sandbox"`
}
