import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Start Here',
      collapsed: false,
      items: ['cli-installation', 'cli-auth', 'cli-overview'],
    },
    {
      type: 'category',
      label: 'Usage',
      collapsed: false,
      items: ['cli-timesheets', 'cli-output-contract', 'cli-roadmap'],
    },
  ],
};

export default sidebars;
