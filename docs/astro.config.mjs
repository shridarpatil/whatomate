import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://shridarpatil.github.io',
  base: '/whatomate',
  integrations: [
    starlight({
      title: 'Whatomate',
      description: 'A modern WhatsApp Business Platform',
      social: {
        github: 'https://github.com/shridarpatil/whatomate',
      },
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
            { label: 'Dashboard', slug: 'features/dashboard' },
            { label: 'Roles & Permissions', slug: 'features/roles-permissions' },
            { label: 'Chatbot Automation', slug: 'features/chatbot' },
            { label: 'Canned Responses', slug: 'features/canned-responses' },
            { label: 'Custom Actions', slug: 'features/custom-actions' },
            { label: 'Templates', slug: 'features/templates' },
            { label: 'Campaigns', slug: 'features/campaigns' },
            { label: 'WhatsApp Flows', slug: 'features/whatsapp-flows' },
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
            { label: 'Roles', slug: 'api-reference/roles' },
            { label: 'Accounts', slug: 'api-reference/accounts' },
            { label: 'Contacts', slug: 'api-reference/contacts' },
            { label: 'Messages', slug: 'api-reference/messages' },
            { label: 'Templates', slug: 'api-reference/templates' },
            { label: 'Flows', slug: 'api-reference/flows' },
            { label: 'Campaigns', slug: 'api-reference/campaigns' },
            { label: 'Chatbot', slug: 'api-reference/chatbot' },
            { label: 'Canned Responses', slug: 'api-reference/canned-responses' },
            { label: 'Custom Actions', slug: 'api-reference/custom-actions' },
            { label: 'Webhooks', slug: 'api-reference/webhooks' },
            { label: 'Analytics', slug: 'api-reference/analytics' },
          ],
        },
      ],
    }),
  ],
});
