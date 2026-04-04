# Pull Request: Whatomate Transactional Email System & Branding Overhaul

This PR introduces a robust, asynchronous transactional email system for the Whatomate platform. It addresses critical communication gaps in user onboarding, security, and platform identity, ensuring a professional "Enterprise-Elite" experience for self-hosted organizations.

## Core Features

### 1. **Fixed Registration & Invitation Workflow**
- **Deep Linking**: Fixed invitation links to correctly route new users to the registration page with their organization context: `{{PublicURL}}/register?org={{OrgID}}`.
- **Credential Delivery**: Improved the `CreateUser` handler to include the invitee's account email and temporary password in the invitation email, ensuring a seamless first-time login.
- **Inviter Context**: Emails now dynamically display the name of the administrator who sent the invitation.

### 2. **Transactional Email Infrastructure**
- **Async Processing**: Integrated `SendEmailAsync` across the `auth` and `user` management flows, leveraging the Redis worker queue to prevent API blocking.
- **Per-Organization SMTP**: Implemented secure extraction and decryption of SMTP credentials from organization-level settings, allowing each tenant to use their own mail server.
- **Preference Enforcement**: Added the `ShouldNotifyUser` helper to ensure all communications respect the individual user settings and "Master Email Toggle" in the UI.

### 3. **"Enterprise Elite" Branding & Design**
- **Platform Integrity**: Conducted a complete branding sweep of the `internal/email/templates/` directory, removing all "BlackloverTech" mentions to establish a pure **Whatomate** identity.
- **Premium Templates**: Redesigned all 8 platform templates (`welcome`, `invite`, `password_reset`, `test`, `alert_no_agent`, `audit_log`, `weekly_report`, `plan_limit`) with a sleek, dark-mode SaaS aesthetic.
- **Standardized Footer**: All platform communications now feature the mandatory footer:
  > Sent by Whatomate · WhatsApp Business API Platform
  > This is an automated message. Please do not reply directly.

---

## Technical Details
- **Handlers**: Updated `internal/handlers/users.go` and `internal/handlers/auth.go` to support new invitation data.
- **Imports**: Fixed missing `fmt` and `uuid` dependencies in modified Go files.
- **Templates**: All templates are stored in `internal/email/templates/` and use the standard Go `text/template` engine.

---

## Verification
- [x] Verified SMTP connection test email functionality.
- [x] Verified registration flow with org-ID correlation.
- [x] Verified template rendering for all 8 notification types.
- [x] Confirmed zero residual "BlackloverTech" branding.

---
**Deployment Readiness**: This PR is optimized for self-hosted environments where high-performance async processing and clear platform-centric branding are critical.
