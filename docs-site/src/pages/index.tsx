import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import Layout from '@theme/Layout';

import styles from './index.module.css';

const quickCommands = [
  'otta auth login --username <username> --password <password>',
  'otta status --format json',
  'otta worktimes options --date 2026-02-20 --format json',
  'otta worktimes list --date 2026-02-20 --format json',
  'otta worktimes read --id <worktime-id> --format json',
  'otta worktimes browse --from 2026-02-20 --to 2026-02-26 --format json',
  'otta worktimes report --from 2026-02-01 --to 2026-02-28 --format csv',
  'otta calendar overview --from 2026-02-01 --to 2026-02-28 --format json',
  'otta absence browse --from 2026-02-01 --to 2026-02-28 --format json',
  'otta absence options --format json',
  'otta absence add --type <absence-type-id> --from 2026-02-20 --to 2026-02-20 --description "sick leave" --format json',
  'otta absence read --id <absence-id> --format json',
  'otta absence update --id <absence-id> --description "sick leave" --format json',
  'otta absence delete --id <absence-id> --format json',
  'otta absence comment --type sick --from 2026-02-20 --to 2026-02-20 --format json',
  'otta holidays --from 2026-02-20 --to 2026-02-20 --worktimegroup <id> --format json',
  'otta holidays read --from 2026-02-20 --to 2026-02-20 --worktimegroup <id> --format json',
];

const installCommands = [
  'brew tap mekedron/tap',
  'brew install mekedron/tap/otta-cli',
];

const highlights = [
  {
    title: 'Credential Modes',
    description:
      'Use local config for interactive workflows or inject credentials with OTTA_CLI_* environment variables for Docker/CI.',
  },
  {
    title: 'Cache Separation',
    description:
      'Only user-entered credentials are persisted in config. API-derived profile data is stored in a separate cache file.',
  },
  {
    title: 'Timesheet Operations',
    description:
      'List/read/browse/report, inspect selectable options, and manage full worktime + absence CRUD flows with deterministic --format output.',
  },
  {
    title: 'Automation Friendly',
    description:
      'Commands are designed for n8n and shell scripts with stable output envelopes and explicit error exits.',
  },
];

const docLinks = [
  {to: '/docs/cli-installation', label: 'Installation', summary: 'Build and run locally, set paths, and bootstrap env vars.'},
  {to: '/docs/cli-auth', label: 'Authentication', summary: 'Credential flow, token handling, and security notes.'},
  {to: '/docs/cli-timesheets', label: 'Time Tracking', summary: 'Worktime list/read/browse/report/CRUD, absence browse/read/add/update/delete, holidays read/retrieval, and calendar overview.'},
  {to: '/docs/cli-output-contract', label: 'Output Contract', summary: 'JSON envelope shape and scriptability guarantees.'},
];

export default function Home() {
  return (
    <Layout
      title="otta-cli"
      description="CLI foundation for automating workflows around otta.fi"
    >
      <header className={clsx('hero hero--primary', styles.heroBanner)}>
        <div className={clsx('container', styles.heroInner)}>
          <p className={styles.kicker}>Practical CLI docs for real workflows</p>
          <Heading as="h1" className={clsx('hero__title', styles.heroTitle)}>
            otta-cli
          </Heading>
          <p className={clsx('hero__subtitle', styles.heroSubtitle)}>
            Automate otta.fi time tracking with repeatable commands, explicit auth
            modes, and script-safe output for pipelines.
          </p>
          <div className={styles.heroActions}>
            <Link className={clsx('button button--secondary button--lg', styles.primaryAction)} to="/docs/cli-installation">
              Start Installation
            </Link>
            <Link className={clsx('button button--outline button--lg', styles.secondaryAction)} to="/docs/cli-timesheets">
              Explore Timesheets
            </Link>
          </div>
        </div>
      </header>
      <main className={styles.mainSection}>
        <section className={clsx('container', styles.block)}>
          <div className={styles.blockHeader}>
            <Heading as="h2">Recommended Install</Heading>
            <p>Homebrew tap is the primary distribution path for releases.</p>
          </div>
          <div className={styles.commandGrid}>
            {installCommands.map((command) => (
              <div key={command} className={styles.commandCard}>
                <code>{command}</code>
              </div>
            ))}
          </div>
        </section>

        <section className={clsx('container', styles.block)}>
          <div className={styles.blockHeader}>
            <Heading as="h2">Quick Command Set</Heading>
            <p>Use these as first-run sanity checks or copy/paste seeds for automation.</p>
          </div>
          <div className={styles.commandGrid}>
            {quickCommands.map((command) => (
              <div key={command} className={styles.commandCard}>
                <code>{command}</code>
              </div>
            ))}
          </div>
        </section>

        <section className={clsx('container', styles.block)}>
          <div className={styles.blockHeader}>
            <Heading as="h2">What This CLI Covers</Heading>
            <p>Focused scope for daily time tracking tasks without hidden state.</p>
          </div>
          <div className={styles.featureGrid}>
            {highlights.map((item) => (
              <article key={item.title} className={styles.featureCard}>
                <Heading as="h3">{item.title}</Heading>
                <p>{item.description}</p>
              </article>
            ))}
          </div>
        </section>

        <section className={clsx('container', styles.block)}>
          <div className={styles.blockHeader}>
            <Heading as="h2">Read Next</Heading>
            <p>Jump directly to the right docs section for your current task.</p>
          </div>
          <div className={styles.docGrid}>
            {docLinks.map((item) => (
              <Link key={item.to} className={styles.docCard} to={item.to}>
                <Heading as="h3">{item.label}</Heading>
                <p>{item.summary}</p>
              </Link>
            ))}
          </div>
        </section>
      </main>
    </Layout>
  );
}
