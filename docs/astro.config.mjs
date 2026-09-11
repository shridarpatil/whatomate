import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://shridarpatil.github.io',
  base: '/whatomate',
  // astro 6 leaves markdown.gfm undefined and lets its own processor default it
  // to on, but @astrojs/mdx still reads the raw value — undefined disables
  // remark-gfm for .mdx, silently rendering every table as literal text.
  // All our content is .mdx, so set it explicitly. Remove once @astrojs/mdx
  // stops reading markdown.gfm.
  markdown: { gfm: true },
  integrations: [
    starlight({
      title: 'Whatomate',
      description: 'A modern WhatsApp Business Platform',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/shridarpatil/whatomate' },
      ],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Introduction', slug: 'getting-started/introduction' },
            { label: 'Quickstart', slug: 'getting-started/quickstart' },
            { label: 'Configuration', slug: 'getting-started/configuration' },
          ],
        },
        {
          label: 'Features',
          items: [
            { label: 'Inbox & Chat', slug: 'features/inbox' },
            { label: 'Dashboard', slug: 'features/dashboard' },
            { label: 'Embedded Signup & Coexistence', slug: 'features/embedded-signup' },
            { label: 'Roles & Permissions', slug: 'features/roles-permissions' },
            { label: 'Teams', slug: 'features/teams' },
            { label: 'SSO (Single Sign-On)', slug: 'features/sso' },
            { label: 'Audit Logs', slug: 'features/audit-logs' },
            { label: 'Chatbot Automation', slug: 'features/chatbot' },
            { label: 'Canned Responses', slug: 'features/canned-responses' },
            { label: 'Custom Actions', slug: 'features/custom-actions' },
            { label: 'Templates', slug: 'features/templates' },
            { label: 'Campaigns', slug: 'features/campaigns' },
            { label: 'WhatsApp Flows', slug: 'features/whatsapp-flows' },
            { label: 'Product Catalogs', slug: 'features/catalogs' },
            { label: 'Calling', slug: 'features/calling' },
            { label: 'Meta Insights', slug: 'features/meta-insights' },
            { label: 'Outbound Webhooks', slug: 'features/webhooks' },
          ],
        },
        {
          label: 'API Reference',
          items: [
            { label: 'Overview', slug: 'api-reference/overview' },
            { label: 'Authentication', slug: 'api-reference/authentication' },
            { label: 'API Keys', slug: 'api-reference/api-keys' },
            { label: 'Users', slug: 'api-reference/users' },
            { label: 'Organizations', slug: 'api-reference/organizations' },
            { label: 'Organization Settings', slug: 'api-reference/org-settings' },
            { label: 'Roles', slug: 'api-reference/roles' },
            { label: 'Teams', slug: 'api-reference/teams' },
            { label: 'Accounts', slug: 'api-reference/accounts' },
            { label: 'Contacts', slug: 'api-reference/contacts' },
            { label: 'Tags', slug: 'api-reference/tags' },
            { label: 'Messages', slug: 'api-reference/messages' },
            { label: 'Templates', slug: 'api-reference/templates' },
            { label: 'Flows', slug: 'api-reference/flows' },
            { label: 'Campaigns', slug: 'api-reference/campaigns' },
            { label: 'Chatbot', slug: 'api-reference/chatbot' },
            { label: 'Canned Responses', slug: 'api-reference/canned-responses' },
            { label: 'Custom Actions', slug: 'api-reference/custom-actions' },
            { label: 'Catalogs', slug: 'api-reference/catalogs' },
            { label: 'Calling', slug: 'api-reference/calling' },
            { label: 'IVR Flows', slug: 'api-reference/ivr-flows' },
            { label: 'Webhooks', slug: 'api-reference/webhooks' },
            { label: 'Analytics', slug: 'api-reference/analytics' },
            { label: 'Dashboard Widgets', slug: 'api-reference/widgets' },
            { label: 'Export & Import', slug: 'api-reference/export-import' },
            { label: 'Audit Logs', slug: 'api-reference/audit-logs' },
          ],
        },
      ],
    }),
  ],
});
