# Meta Embedded Signup with Coexistence

This feature enables users to sign up for WhatsApp Business API directly through your website using Meta's Embedded Signup flow with full coexistence support.

## Overview

**Embedded Signup** is Meta's streamlined OAuth-based onboarding flow that allows users to:
- Connect their existing WhatsApp Business App number to the Cloud API
- Automatically configure webhooks and phone number settings
- Sync up to 6 months of chat history (with coexistence enabled)
- Use the same number on both Business App and API simultaneously

**Coexistence** allows businesses to use the same phone number on both WhatsApp Business App and WhatsApp Cloud API, with messages synced across both platforms using Meta's "Messaging Echoes" system.

## Features

- ✅ **Meta Graph API v24.0** - Latest version with full coexistence support
- ✅ **OAuth 2.0 Flow** - Secure Meta authentication
- ✅ **Coexistence Support** - Sync WhatsApp Business App with Cloud API
- ✅ **Chat History Sync** - Automatically sync up to 6 months of chats
- ✅ **Auto Contact Creation** - Automatically create contacts from signups
- ✅ **Custom Form Fields** - Collect additional data during signup
- ✅ **Welcome Messages** - Send automated welcome messages
- ✅ **Webhook Notifications** - Get notified when users sign up
- ✅ **CORS Support** - Embed signup widget on any domain
- ✅ **Rate Limiting** - Protect against abuse
- ✅ **Team Assignment** - Automatically assign new leads to teams

## Setup

### 1. Configure Meta App

1. Go to [Meta for Developers](https://developers.facebook.com/)
2. Create or select your WhatsApp Business Platform app
3. Navigate to **WhatsApp > Configuration**
4. Set up **Embedded Signup**:
   - Copy the **Configuration ID**
   - Add your application's domains to **Allowed Domains for the JavaScript SDK**
   - Add OAuth redirect URIs to **Valid OAuth Redirect URIs**
   - Ensure `whatsapp_business_management` permission is enabled

### 2. Create Embedded Signup Configuration

**API Endpoint:** `POST /api/embedded-signups`

**Required Permissions:** Manager or Admin

**Request Body:**
```json
{
  "name": "Website Signup Form",
  "whatsapp_account_id": "uuid-of-default-account",
  "meta_app_id": "your-meta-app-id",
  "meta_config_id": "your-configuration-id",
  "meta_app_secret": "your-app-secret",
  "enable_coexistence": true,
  "sync_chat_history": true,
  "api_version": "v24.0",
  "form_fields": {
    "company_name": {
      "type": "text",
      "label": "Company Name",
      "required": true
    },
    "industry": {
      "type": "select",
      "label": "Industry",
      "options": ["E-commerce", "Healthcare", "Education", "Other"]
    }
  },
  "required_fields": ["phone", "company_name"],
  "welcome_message": "Welcome! Thank you for signing up.",
  "success_message": "Your WhatsApp Business account has been connected successfully!",
  "allowed_origins": ["https://yourwebsite.com", "https://app.yourwebsite.com"],
  "rate_limit_per_hour": 100,
  "is_active": true,
  "auto_create_contact": true
}
```

**Response:**
```json
{
  "id": "uuid-of-signup-config",
  "name": "Website Signup Form",
  "meta_app_id": "your-meta-app-id",
  "meta_config_id": "your-configuration-id",
  "enable_coexistence": true,
  "sync_chat_history": true,
  "api_version": "v24.0",
  "created_at": "2026-01-10T12:00:00Z"
}
```

### 3. Embed Signup Widget

#### Get Public Configuration

**API Endpoint:** `GET /api/embedded-signup/{id}/config`

**Public Endpoint** - No authentication required

**Response:**
```json
{
  "id": "signup-config-uuid",
  "name": "Website Signup Form",
  "meta_app_id": "your-meta-app-id",
  "meta_config_id": "your-configuration-id",
  "enable_coexistence": true,
  "form_fields": { ... },
  "required_fields": ["phone", "company_name"],
  "success_message": "Success!",
  "redirect_url": null
}
```

#### Example HTML Integration

```html
<!DOCTYPE html>
<html>
<head>
    <title>WhatsApp Business Signup</title>
    <script src="https://connect.facebook.net/en_US/sdk.js"></script>
</head>
<body>
    <h1>Connect Your WhatsApp Business</h1>
    <button id="signup-btn">Sign Up with WhatsApp</button>

    <script>
        const SIGNUP_ID = 'your-signup-config-uuid';
        const API_URL = 'https://your-whatomate-instance.com';

        // Fetch public config
        fetch(`${API_URL}/api/embedded-signup/${SIGNUP_ID}/config`)
            .then(res => res.json())
            .then(config => {
                // Initialize Facebook SDK
                window.fbAsyncInit = function() {
                    FB.init({
                        appId: config.meta_app_id,
                        cookie: true,
                        xfbml: true,
                        version: config.api_version
                    });
                };

                // Handle signup button click
                document.getElementById('signup-btn').addEventListener('click', () => {
                    // Launch Embedded Signup flow
                    FB.login(function(response) {
                        if (response.authResponse) {
                            const authCode = response.authResponse.code;

                            // Submit signup with auth code
                            fetch(`${API_URL}/api/embedded-signup/${SIGNUP_ID}/submit`, {
                                method: 'POST',
                                headers: {
                                    'Content-Type': 'application/json'
                                },
                                body: JSON.stringify({
                                    phone_number: '+1234567890', // User's phone
                                    profile_name: 'Company Name',
                                    meta_auth_code: authCode,
                                    form_data: {
                                        company_name: 'Acme Corp',
                                        industry: 'E-commerce'
                                    },
                                    source: 'website'
                                })
                            })
                            .then(res => res.json())
                            .then(result => {
                                if (result.success) {
                                    alert(result.message);
                                    if (result.redirect_url) {
                                        window.location.href = result.redirect_url;
                                    }
                                }
                            });
                        }
                    }, {
                        config_id: config.meta_config_id,
                        response_type: 'code',
                        override_default_response_type: true,
                        extras: {
                            setup: {
                                ... // Optional setup parameters
                            }
                        }
                    });
                });
            });
    </script>
</body>
</html>
```

## API Reference

### Management Endpoints (Manager+ Required)

#### List All Signups
```
GET /api/embedded-signups
```

#### Create Signup Configuration
```
POST /api/embedded-signups
```

#### Get Signup Details
```
GET /api/embedded-signups/{id}
```

#### Update Signup Configuration
```
PUT /api/embedded-signups/{id}
```

#### Delete Signup Configuration
```
DELETE /api/embedded-signups/{id}
```

#### List Leads
```
GET /api/embedded-signups/{id}/leads
```

### Public Endpoints (No Auth Required)

#### Get Public Configuration
```
GET /api/embedded-signup/{id}/config
```

#### Submit Signup
```
POST /api/embedded-signup/{id}/submit
```

Request body:
```json
{
  "phone_number": "+1234567890",
  "profile_name": "John Doe",
  "meta_auth_code": "oauth-code-from-meta",
  "form_data": {
    "company_name": "Acme Corp",
    "custom_field": "value"
  },
  "source": "widget"
}
```

## Coexistence Details

### What is Coexistence?

Coexistence allows businesses to use the same WhatsApp number on both:
- **WhatsApp Business App** (mobile app)
- **WhatsApp Cloud API** (programmatic access)

### Key Benefits

1. **No Number Migration** - Keep using your existing Business App number
2. **Unified Inbox** - Messages from both channels appear in both interfaces
3. **Chat History Sync** - Up to 6 months of chat history synced automatically
4. **Gradual Transition** - Move to API at your own pace while keeping the app

### Requirements

- WhatsApp Business App version **2.24.17 or higher**
- Phone number from a [supported country](https://developers.facebook.com/docs/whatsapp/cloud-api/overview/coexistence)
- Embedded Signup with verified session logging

### How It Works

1. User initiates Embedded Signup from your website
2. Meta authenticates and links their Business App number
3. Webhooks are automatically configured
4. Chat history is synced (if enabled)
5. Messages sent via the API appear in the Business App
6. Messages sent via the Business App appear in the API inbox

### Messaging Echoes

When coexistence is enabled, messages are echoed between platforms:
- **App → API**: Messages sent via Business App trigger webhook events
- **API → App**: Messages sent via API appear in Business App inbox

## Security

### CORS Configuration

Specify allowed origins in the signup configuration:

```json
{
  "allowed_origins": [
    "https://yoursite.com",
    "https://app.yoursite.com",
    "*.yourdomain.com"  // Wildcard subdomains
  ]
}
```

### Webhook Signatures

Webhook notifications include HMAC-SHA256 signatures for verification:

```
X-Whatomate-Signature: hex-encoded-hmac-sha256
```

Verify using the `meta_app_secret` as the key.

### Rate Limiting

Configure rate limits per signup:

```json
{
  "rate_limit_per_hour": 100
}
```

## Webhook Events

Subscribe to embedded signup events:

```json
{
  "webhook_url": "https://yoursite.com/webhooks/signup"
}
```

Event payload:
```json
{
  "event": "embedded_signup.lead_created",
  "lead_id": "uuid",
  "phone": "+1234567890",
  "name": "John Doe",
  "form_data": { ... },
  "status": "confirmed",
  "source": "widget",
  "created_at": "2026-01-10T12:00:00Z"
}
```

## Best Practices

1. **Always Enable Coexistence** - Unless you specifically need to migrate away from the Business App
2. **Sync Chat History** - Provides better context for API-based interactions
3. **Use Welcome Messages** - Engage users immediately after signup
4. **Collect Minimal Data** - Only ask for essential information in custom fields
5. **Monitor Rate Limits** - Adjust based on your traffic patterns
6. **Verify Webhook Signatures** - Always validate incoming webhook requests
7. **Handle Errors Gracefully** - Provide clear error messages to users

## Troubleshooting

### Signup Fails

- Verify `meta_app_id` and `meta_config_id` are correct
- Check that the user's phone number is from a supported country
- Ensure the Business App is version 2.24.17 or higher
- Verify OAuth redirect URIs are correctly configured in Meta

### Chat History Not Syncing

- Ensure `sync_chat_history` is set to `true`
- Verify the user completed the Meta authentication flow
- Check that the WhatsApp Business App has sufficient chat history (up to 6 months)

### CORS Errors

- Add the embedding domain to `allowed_origins`
- Use exact domain match (including protocol: `https://`)
- For development, use `["*"]` temporarily (not for production)

### Webhook Not Receiving Events

- Verify `webhook_url` is publicly accessible
- Check webhook signature validation
- Ensure the endpoint returns 200 OK quickly

## API Version Support

| API Version | Coexistence | Status |
|-------------|-------------|--------|
| v24.0       | ✅ Full     | Latest (Recommended) |
| v23.0       | ✅ Full     | Supported |
| v22.0       | ✅ Full     | Supported |
| v21.0       | ⚠️ Partial  | Legacy |
| < v21.0     | ❌ No       | Not Recommended |

**Recommendation:** Use v24.0 or later for best coexistence support.

## Resources

- [Meta WhatsApp Embedded Signup Docs](https://developers.facebook.com/docs/whatsapp/embedded-signup)
- [WhatsApp Coexistence Guide](https://developers.facebook.com/docs/whatsapp/cloud-api/overview/coexistence)
- [Meta Graph API Reference](https://developers.facebook.com/docs/graph-api)
- [WhatsApp Cloud API Overview](https://developers.facebook.com/docs/whatsapp/cloud-api)

## Support

For issues or questions:
1. Check the troubleshooting section above
2. Review Meta's official documentation
3. Open an issue on GitHub
4. Contact support at your Whatomate instance
