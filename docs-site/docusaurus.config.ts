import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'otta-cli',
  tagline: 'CLI foundation for automating otta.fi time tracking workflows',

  future: {
    v4: true,
  },

  url: 'https://mekedron.github.io',
  baseUrl: '/otta-cli/',

  organizationName: 'mekedron',
  projectName: 'otta-cli',

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    mermaid: true,
  },
  themes: ['@docusaurus/theme-mermaid'],

  presets: [
    [
      'classic',
      {
        docs: {
          path: '../docs',
          routeBasePath: 'docs',
          sidebarPath: './sidebars.ts',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'otta-cli',
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          href: 'https://github.com/mekedron/otta-cli',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'light',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'Installation',
              to: '/docs/cli-installation',
            },
            {
              label: 'Authentication',
              to: '/docs/cli-auth',
            },
            {
              label: 'Overview',
              to: '/docs/cli-overview',
            },
            {
              label: 'Roadmap',
              to: '/docs/cli-roadmap',
            },
          ],
        },
        {
          title: 'Project',
          items: [
            {
              label: 'Repository',
              href: 'https://github.com/mekedron/otta-cli',
            },
            {
              label: 'Security',
              href: 'https://github.com/mekedron/otta-cli/blob/main/SECURITY.md',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} otta-cli contributors.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
