/**
 * Universal SaaS: Standard Engineering Error Taxonomy
 * This library maps internal ERR_ codes to industrial-grade remediation strategies.
 */

export type ErrorSeverity = 'INFO' | 'WARNING' | 'ERROR' | 'CRITICAL';

export interface DiagnosticInfo {
  code: string;
  message: string;
  remediation: string;
  severity: ErrorSeverity;
  domain: string;
  actionable: boolean;
}

const ERROR_MAP: Record<string, Omit<DiagnosticInfo, 'code'>> = {
  // REST & Request Validation
  'ERR_REST_UNAUTHORIZED': {
    message: 'Authentication session invalid or expired.',
    remediation: 'Please re-authenticate to restore secure access.',
    severity: 'ERROR',
    domain: 'auth',
    actionable: true,
  },
  'ERR_REST_FORBIDDEN': {
    message: 'Access denied: Insufficient privileges for this resource.',
    remediation: 'Contact your organization administrator to upgrade your Role Permissions.',
    severity: 'ERROR',
    domain: 'auth',
    actionable: false,
  },

  // WhatsApp Business Platform
  'ERR_WA_OUT_OF_WINDOW': {
    message: 'The 24-hour customer service window has expired.',
    remediation: 'Standard messages are blocked. Please send a pre-approved WhatsApp Template to re-engage.',
    severity: 'WARNING',
    domain: 'whatsapp',
    actionable: true,
  },
  'ERR_WA_SPAM_BLOCK': {
    message: 'Outbound messaging suspended due to high spam reports.',
    remediation: 'Messaging is frozen. Review your content quality and appeal in Meta Business Manager.',
    severity: 'CRITICAL',
    domain: 'whatsapp',
    actionable: true,
  },
  'ERR_WA_TOKEN_EXPIRED': {
    message: 'Meta Graph API Access Token has expired.',
    remediation: 'Reconnect your WhatsApp Business Account in Settings to renew the handshake.',
    severity: 'CRITICAL',
    domain: 'whatsapp',
    actionable: true,
  },

  // Platform Core
  'ERR_PLATFORM_AUTH_RBAC_DENIED': {
    message: 'RBAC Security Violation: Access to this specific operation is restricted.',
    remediation: 'This incident has been logged. Verify your administrative tier with the system owner.',
    severity: 'CRITICAL',
    domain: 'security',
    actionable: false,
  },
  'ERR_PLATFORM_CONTACT_DUPLICATE': {
    message: 'A contact with this identity already exists.',
    remediation: 'Use the "Merge Records" tool to unify the chat history.',
    severity: 'INFO',
    domain: 'crm',
    actionable: true,
  },

  // Chatbot & AI
  'ERR_BOT_SESSION_EXPIRED': {
    message: 'Active chatbot context has timed out due to inactivity.',
    remediation: 'The bot session will restart from the initial Greeting Node on the next interaction.',
    severity: 'INFO',
    domain: 'bot',
    actionable: false,
  },
};

/**
 * Resolves a high-fidelity diagnostic record for any platform error code.
 */
export function getDiagnosticInfo(errorCode: string): DiagnosticInfo {
  const info = ERROR_MAP[errorCode];
  
  if (!info) {
    return {
      code: errorCode,
      message: 'An undocumented internal error occurred.',
      remediation: 'Capture the meta_trace_id and escalate to Platform Engineering.',
      severity: 'ERROR',
      domain: 'system',
      actionable: false,
    };
  }

  return {
    code: errorCode,
    ...info,
  };
}
