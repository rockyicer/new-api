import test from 'node:test';
import assert from 'node:assert/strict';

import {
  PERSONAL_SIDEBAR_ADMIN_ONLY_SECTIONS,
  buildFinalSidebarConfig,
  filterUserEditableSidebarSections,
  stripAdminOnlySectionsFromUserSidebarConfig,
} from './sidebar-config.js';

const adminConfig = {
  chat: {
    enabled: true,
    playground: true,
    chat: true,
  },
  console: {
    enabled: true,
    detail: true,
    token: true,
    log: true,
    midjourney: true,
    task: true,
  },
  personal: {
    enabled: true,
    topup: true,
    personal: true,
  },
  admin: {
    enabled: true,
    setting: true,
  },
};

test('buildFinalSidebarConfig ignores user toggles for admin-only personal sections', () => {
  const finalConfig = buildFinalSidebarConfig(adminConfig, {
    chat: {
      enabled: false,
      playground: false,
      chat: false,
    },
    console: {
      enabled: false,
      midjourney: false,
      task: false,
    },
    personal: {
      enabled: false,
      topup: false,
      personal: false,
    },
    admin: {
      enabled: true,
      setting: false,
    },
  });

  assert.equal(finalConfig.chat.enabled, true);
  assert.equal(finalConfig.chat.playground, true);
  assert.equal(finalConfig.chat.chat, true);
  assert.equal(finalConfig.console.enabled, true);
  assert.equal(finalConfig.console.midjourney, true);
  assert.equal(finalConfig.console.task, true);
  assert.equal(finalConfig.personal.enabled, true);
  assert.equal(finalConfig.personal.topup, true);
  assert.equal(finalConfig.personal.personal, true);
  assert.equal(finalConfig.admin.enabled, true);
  assert.equal(finalConfig.admin.setting, false);
});

test('stripAdminOnlySectionsFromUserSidebarConfig removes admin-only personal sections before save', () => {
  const stripped = stripAdminOnlySectionsFromUserSidebarConfig({
    chat: { enabled: false, playground: false },
    console: { enabled: false, task: false },
    personal: { enabled: false, topup: false },
    admin: { enabled: true, setting: false },
  });

  assert.deepEqual(stripped, {
    admin: { enabled: true, setting: false },
  });
});

test('filterUserEditableSidebarSections hides admin-only personal sections from personal settings UI', () => {
  const filtered = filterUserEditableSidebarSections([
    { key: 'chat', title: 'chat' },
    { key: 'console', title: 'console' },
    { key: 'personal', title: 'personal' },
    { key: 'admin', title: 'admin' },
  ]);

  assert.deepEqual(
    filtered.map((section) => section.key),
    ['admin'],
  );
  assert.deepEqual(PERSONAL_SIDEBAR_ADMIN_ONLY_SECTIONS, [
    'chat',
    'console',
    'personal',
  ]);
});
