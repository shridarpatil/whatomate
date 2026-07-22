import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../../helpers';
import { AccountsPage } from '../../pages';

test.describe('WhatsApp Business Profile', () => {
    let accountsPage: AccountsPage;

    test.beforeEach(async ({ page }) => {
        await loginAsAdmin(page);
        accountsPage = new AccountsPage(page);

        // Mock the GET /accounts to ensure we have a test subject
        await page.route('**/api/accounts', async route => {
            if (route.request().method() === 'GET') {
                await route.fulfill({
                    status: 200,
                    contentType: 'application/json',
                    body: JSON.stringify({
                        data: {
                            accounts: [{
                                id: 'test-acc-id',
                                name: 'Test Account',
                                phone_id: '123456',
                                business_id: '789012',
                                status: 'active'
                            }]
                        }
                    })
                });
            } else {
                await route.continue();
            }
        });

        // Mock GET single account (for detail page)
        await page.route('**/api/accounts/test-acc-id', async route => {
            if (route.request().method() === 'GET') {
                await route.fulfill({
                    status: 200,
                    contentType: 'application/json',
                    body: JSON.stringify({
                        data: {
                            id: 'test-acc-id',
                            name: 'Test Account',
                            phone_id: '123456',
                            business_id: '789012',
                            api_version: 'v21.0',
                            webhook_verify_token: 'abc123',
                            status: 'active',
                            has_access_token: true,
                            has_app_secret: false,
                            is_default_incoming: false,
                            is_default_outgoing: false,
                            auto_read_receipt: false,
                            created_at: '2026-01-01T00:00:00Z',
                            updated_at: '2026-01-01T00:00:00Z'
                        }
                    })
                });
            } else {
                await route.continue();
            }
        });

        // Mock GET profile
        await page.route('**/api/accounts/*/business_profile*', async route => {
            if (route.request().method() === 'GET') {
                await route.fulfill({
                    status: 200,
                    contentType: 'application/json',
                    body: JSON.stringify({
                        data: {
                            about: 'Available',
                            address: '123 Test St',
                            description: 'Test Business',
                            email: 'test@example.com',
                            vertical: 'PROF_SERVICES',
                            websites: ['https://example.com'],
                            profile_picture_url: ''
                        }
                    })
                });
            } else {
                await route.continue();
            }
        });

        // Mock audit logs
        await page.route('**/api/audit-logs*', async route => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({ data: { audit_logs: [], total: 0 } })
            });
        });
    });

    test('should view business profile dialog', async ({ page }) => {
        // Navigate to account detail page
        await page.goto('/settings/accounts/test-acc-id');
        await page.waitForLoadState('networkidle');

        // Open business profile dialog
        await accountsPage.openBusinessProfile();
        await accountsPage.expectProfileDialogVisible();

        // Verify fields contain mocked data
        await expect(accountsPage.profileDialog.locator('input#about')).toHaveValue('Available');
        await expect(accountsPage.profileDialog.locator('input#email')).toHaveValue('test@example.com');
    });

    test('should update business profile', async ({ page }) => {
        await page.goto('/settings/accounts/test-acc-id');
        await page.waitForLoadState('networkidle');

        await accountsPage.openBusinessProfile();

        // Mock PUT request
        await page.route('**/api/accounts/*/business_profile', async route => {
            if (route.request().method() === 'PUT') {
                await route.fulfill({
                    status: 200,
                    contentType: 'application/json',
                    body: JSON.stringify({
                        data: { success: true }
                    })
                });
            } else {
                await route.continue();
            }
        });

        // Change value
        await accountsPage.profileDialog.locator('input#about').fill('Busy');
        await accountsPage.profileDialog.getByRole('button', { name: 'Save Changes' }).click();

        // Verify success toast
        await accountsPage.expectToast(/updated successfully/i);
    });
});
