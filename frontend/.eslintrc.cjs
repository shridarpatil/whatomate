/* eslint-env node */
require('@rushstack/eslint-patch/modern-module-resolution')

module.exports = {
  root: true,
  extends: [
    'plugin:vue/vue3-essential',
    'eslint:recommended',
    '@vue/eslint-config-typescript',
    '@vue/eslint-config-prettier/skip-formatting'
  ],
  parserOptions: {
    ecmaVersion: 'latest'
  },
  ignorePatterns: ['dist/', 'node_modules/', 'playwright-report/', 'test-results/', 'src/vendor/'],
  rules: {
    'vue/multi-word-component-names': 'off',
    '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
    '@typescript-eslint/no-explicit-any': 'off',
    'no-inner-declarations': 'off'
  },
  overrides: [
    {
      files: ['*.cjs', 'scripts/*.mjs'],
      env: {
        node: true,
        commonjs: true
      }
    }
  ]
}
