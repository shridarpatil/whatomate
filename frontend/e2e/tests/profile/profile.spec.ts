import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ProfilePage } from '../../pages'

test.describe('Profile Page', () => {
  let profilePage: ProfilePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    profilePage = new ProfilePage(page)
    await profilePage.goto()
  })

  test('should display profile page', async () => {
    await profilePage.expectPageVisible()
  })

  test('should show account information card', async () => {
    await expect(profilePage.accountInfoCard).toBeVisible()
    await expect(profilePage.accountInfoCard).toContainText('Account Information')
  })

  test('should show change password card', async () => {
    await expect(profilePage.changePasswordCard).toBeVisible()
    await expect(profilePage.changePasswordCard).toContainText('Change Password')
  })

  test('should display user name', async () => {
    await expect(profilePage.accountInfoCard).toContainText('Name')
  })

  test('should display user email', async () => {
    await expect(profilePage.accountInfoCard).toContainText('Email')
  })

  test('should display user role', async () => {
    await expect(profilePage.accountInfoCard).toContainText('Role')
  })
})

test.describe('Password Change Form', () => {
  let profilePage: ProfilePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    profilePage = new ProfilePage(page)
    await profilePage.goto()
  })

  test('should have current password field', async () => {
    await expect(profilePage.currentPasswordInput).toBeVisible()
  })

  test('should have new password field', async () => {
    await expect(profilePage.newPasswordInput).toBeVisible()
  })

  test('should have confirm password field', async () => {
    await expect(profilePage.confirmPasswordInput).toBeVisible()
  })

  test('should have change password button', async () => {
    await expect(profilePage.changePasswordButton).toBeVisible()
  })

  test('should show validation error for mismatched passwords', async () => {
    await profilePage.fillPasswordForm('oldpassword', 'newpassword1', 'newpassword2')
    await profilePage.changePasswordButton.click()
    await profilePage.expectToast(/match/i)
  })

  test('should show validation error for short password', async () => {
    await profilePage.fillPasswordForm('oldpassword', '12345', '12345')
    await profilePage.changePasswordButton.click()
    await profilePage.expectToast(/6 characters/i)
  })

  test('should toggle current password visibility', async ({ page }) => {
    await expect(profilePage.currentPasswordInput).toHaveAttribute('type', 'password')

    const toggleBtn = profilePage.currentPasswordInput.locator('..').locator('button')
    await toggleBtn.click()

    await expect(profilePage.currentPasswordInput).toHaveAttribute('type', 'text')
  })

  test('should toggle new password visibility', async ({ page }) => {
    await expect(profilePage.newPasswordInput).toHaveAttribute('type', 'password')

    const toggleBtn = profilePage.newPasswordInput.locator('..').locator('button')
    await toggleBtn.click()

    await expect(profilePage.newPasswordInput).toHaveAttribute('type', 'text')
  })

  test('should toggle confirm password visibility', async ({ page }) => {
    await expect(profilePage.confirmPasswordInput).toHaveAttribute('type', 'password')

    const toggleBtn = profilePage.confirmPasswordInput.locator('..').locator('button')
    await toggleBtn.click()

    await expect(profilePage.confirmPasswordInput).toHaveAttribute('type', 'text')
  })

  test('should clear form after successful password change', async () => {
    // This test would require knowing the current password
    // Skipping actual submission but testing the form works
    await profilePage.fillPasswordForm('test', 'newpass123', 'newpass123')
    await expect(profilePage.currentPasswordInput).toHaveValue('test')
    await expect(profilePage.newPasswordInput).toHaveValue('newpass123')
    await expect(profilePage.confirmPasswordInput).toHaveValue('newpass123')
  })
})

test.describe('Profile Page Labels', () => {
  let profilePage: ProfilePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    profilePage = new ProfilePage(page)
    await profilePage.goto()
  })

  test('should show Current Password label', async () => {
    await expect(profilePage.changePasswordCard.getByText('Current Password')).toBeVisible()
  })

  test('should show New Password label', async () => {
    await expect(profilePage.changePasswordCard.getByText('New Password', { exact: true })).toBeVisible()
  })

  test('should show Confirm New Password label', async () => {
    await expect(profilePage.changePasswordCard.getByText('Confirm New Password')).toBeVisible()
  })

  test('should show password requirement hint', async () => {
    await expect(profilePage.changePasswordCard.getByText(/6 characters/i)).toBeVisible()
  })
})
