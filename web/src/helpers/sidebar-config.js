const deepClone = (value) => JSON.parse(JSON.stringify(value));

export const PERSONAL_SIDEBAR_ADMIN_ONLY_SECTIONS = [
  'chat',
  'console',
  'personal',
];

const personalSidebarAdminOnlySectionSet = new Set(
  PERSONAL_SIDEBAR_ADMIN_ONLY_SECTIONS,
);

export const isPersonalSidebarAdminOnlySection = (sectionKey) =>
  personalSidebarAdminOnlySectionSet.has(sectionKey);

export const stripAdminOnlySectionsFromUserSidebarConfig = (userConfig) => {
  if (!userConfig || typeof userConfig !== 'object') {
    return {};
  }

  return Object.entries(userConfig).reduce((result, [sectionKey, sectionValue]) => {
    if (isPersonalSidebarAdminOnlySection(sectionKey)) {
      return result;
    }

    result[sectionKey] = deepClone(sectionValue);
    return result;
  }, {});
};

export const filterUserEditableSidebarSections = (sectionConfigs) => {
  if (!Array.isArray(sectionConfigs)) {
    return [];
  }

  return sectionConfigs.filter(
    (section) => !isPersonalSidebarAdminOnlySection(section?.key),
  );
};

export const buildFinalSidebarConfig = (adminConfig, userConfig) => {
  const result = {};

  if (!adminConfig || typeof adminConfig !== 'object') {
    return result;
  }

  Object.keys(adminConfig).forEach((sectionKey) => {
    const adminSection = adminConfig[sectionKey];
    const shouldIgnoreUserConfig =
      isPersonalSidebarAdminOnlySection(sectionKey);
    const userSection = shouldIgnoreUserConfig
      ? null
      : userConfig?.[sectionKey];

    if (!adminSection?.enabled) {
      result[sectionKey] = { enabled: false };
      return;
    }

    const sectionEnabled = shouldIgnoreUserConfig
      ? true
      : userSection
        ? userSection.enabled !== false
        : true;
    result[sectionKey] = { enabled: sectionEnabled };

    Object.keys(adminSection).forEach((moduleKey) => {
      if (moduleKey === 'enabled') {
        return;
      }

      const adminAllowed = adminSection[moduleKey];
      const userAllowed =
        shouldIgnoreUserConfig || !userSection
          ? true
          : userSection[moduleKey] !== false;

      result[sectionKey][moduleKey] =
        adminAllowed && userAllowed && sectionEnabled;
    });
  });

  return result;
};
