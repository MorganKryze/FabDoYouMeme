<!-- frontend/src/routes/(public)/privacy/+page.svelte -->
<script lang="ts">
  import { env } from '$env/dynamic/public';
  import { reveal } from '$lib/actions/reveal';

  const LAST_UPDATED = '2026-04-13';
  const VERSION = '1.0';

  const fallback = (name: string) => `[${name} not set — see docs/self-hosting.md]`;

  const operator = {
    name: env.PUBLIC_OPERATOR_NAME || fallback('PUBLIC_OPERATOR_NAME'),
    email: env.PUBLIC_OPERATOR_CONTACT_EMAIL || fallback('PUBLIC_OPERATOR_CONTACT_EMAIL'),
    url: env.PUBLIC_OPERATOR_URL || fallback('PUBLIC_OPERATOR_URL'),
    smtp: env.PUBLIC_OPERATOR_SMTP_PROVIDER || fallback('PUBLIC_OPERATOR_SMTP_PROVIDER'),
  };

  const missingVars: string[] = [];
  if (!env.PUBLIC_OPERATOR_NAME) missingVars.push('PUBLIC_OPERATOR_NAME');
  if (!env.PUBLIC_OPERATOR_CONTACT_EMAIL) missingVars.push('PUBLIC_OPERATOR_CONTACT_EMAIL');
  if (!env.PUBLIC_OPERATOR_SMTP_PROVIDER) missingVars.push('PUBLIC_OPERATOR_SMTP_PROVIDER');
</script>

<svelte:head>
  <title>Privacy Policy — FabDoYouMeme</title>
  <meta name="description" content="Privacy policy for this FabDoYouMeme instance. GDPR Art. 13 disclosure." />
</svelte:head>

<article class="mx-auto max-w-3xl w-full flex flex-col gap-8" use:reveal>
  <header class="flex flex-col gap-3 pb-6 border-b-[2.5px] border-brand-border-heavy">
    <h1 class="text-4xl font-bold tracking-tight">Privacy Policy</h1>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      Written to be <strong>simple, readable, and accurate</strong>: plain language over
      legalese, only the data actually collected, only the processors actually used.
      Nothing is padded for the sake of looking complete. Operator-specific fields
      (controller name, contact email, deployment URL, email provider) are injected
      at runtime from environment variables set by whoever runs this instance — so
      every self-hosted deployment shows its own details here automatically.
    </p>
    <p class="text-xs text-brand-text-muted">
      Last updated: {LAST_UPDATED} · Version {VERSION}
    </p>
  </header>

  {#if missingVars.length > 0}
    <div
      role="alert"
      class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-accent/20 px-5 py-4 text-sm font-semibold leading-relaxed"
    >
      <strong>Operator configuration incomplete.</strong>
      The following environment variables are not set:
      {#each missingVars as name, i}<code class="font-mono">{name}</code>{#if i < missingVars.length - 1}, {/if}{/each}.
      See <code>docs/self-hosting.md → Legal / privacy policy</code> for the full list.
      This page is not GDPR-compliant until they are configured.
    </div>
  {/if}

  <!-- §1 Controller identity -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">1. Controller identity (Art. 13(1)(a))</h2>
    <ul class="text-sm font-semibold text-brand-text-muted space-y-2 leading-relaxed">
      <li><strong>Data controller:</strong> {operator.name}</li>
      <li>
        <strong>Contact email:</strong>
        <a href="mailto:{operator.email}" class="underline hover:text-brand-text transition-colors">{operator.email}</a>
        — used for all privacy requests (access, erasure, rectification, objection, complaints).
      </li>
      <li><strong>Hosted at:</strong> {operator.url}</li>
    </ul>
  </section>

  <!-- §2 Purposes -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">2. Purposes and lawful bases (Art. 13(1)(c))</h2>
    <div class="overflow-x-auto">
      <table class="w-full text-sm text-left border-collapse">
        <thead class="text-[0.7rem] uppercase tracking-[0.1em] text-brand-text-muted">
          <tr>
            <th class="py-2 pr-4 font-bold">Purpose</th>
            <th class="py-2 pr-4 font-bold">Data used</th>
            <th class="py-2 font-bold">Lawful basis</th>
          </tr>
        </thead>
        <tbody class="text-brand-text-muted font-semibold">
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Authentication (magic link login)</td>
            <td class="py-2 pr-4">Email address</td>
            <td class="py-2">Contract — Art. 6(1)(b)</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Account management</td>
            <td class="py-2 pr-4">Username, email</td>
            <td class="py-2">Consent — Art. 6(1)(a)</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Game history and leaderboards</td>
            <td class="py-2 pr-4">Submissions, votes, scores</td>
            <td class="py-2">Consent — Art. 6(1)(a)</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Security monitoring</td>
            <td class="py-2 pr-4">Operational logs</td>
            <td class="py-2">Legitimate interest — Art. 6(1)(f)</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Admin accountability</td>
            <td class="py-2 pr-4">Audit log</td>
            <td class="py-2">Legitimate interest — Art. 6(1)(f)</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <!-- §3 Categories -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">3. Categories of personal data collected</h2>
    <ul class="text-sm font-semibold text-brand-text-muted list-disc list-inside space-y-1 leading-relaxed">
      <li><strong>Email address</strong> — used to send authentication links; not shared with other players.</li>
      <li><strong>Username</strong> — displayed in-game and on leaderboards; visible to all players in a room.</li>
      <li><strong>Consent timestamp</strong> — recorded when you accept this policy at registration.</li>
      <li><strong>Game submissions</strong> — captions or answers you submit; visible to room players.</li>
      <li><strong>Game scores</strong> — points earned per game; visible on leaderboards.</li>
      <li><strong>Session cookie</strong> — one <code>HttpOnly</code> functional cookie for authentication; no tracking.</li>
    </ul>
  </section>

  <!-- §4 Retention -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">4. Data retention (Art. 13(2)(a))</h2>
    <div class="overflow-x-auto">
      <table class="w-full text-sm text-left border-collapse">
        <thead class="text-[0.7rem] uppercase tracking-[0.1em] text-brand-text-muted">
          <tr>
            <th class="py-2 pr-4 font-bold">Data</th>
            <th class="py-2 font-bold">Retained for</th>
          </tr>
        </thead>
        <tbody class="text-brand-text-muted font-semibold">
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Account (email, username)</td>
            <td class="py-2">Until you request deletion</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Game history (rooms, submissions, scores)</td>
            <td class="py-2">2 years after the game ends</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Session cookie</td>
            <td class="py-2">30 days, renewed on each visit</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Authentication tokens</td>
            <td class="py-2">15 minutes, single-use</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Operational logs</td>
            <td class="py-2">Up to 30 days (automatic rotation)</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">Database backups</td>
            <td class="py-2">7 days after deletion (see §10)</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <!-- §5 Rights -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">5. Your rights (Art. 13(2)(b))</h2>
    <ul class="text-sm font-semibold text-brand-text-muted space-y-2 leading-relaxed">
      <li><strong>Access (Art. 15)</strong> — download your data at Profile → "Download My Data".</li>
      <li><strong>Portability (Art. 20)</strong> — same as above; downloads as JSON.</li>
      <li><strong>Rectification (Art. 16)</strong> — update username or email at Profile.</li>
      <li><strong>Erasure (Art. 17)</strong> — email <a href="mailto:{operator.email}" class="underline">{operator.email}</a>, processed within 30 days.</li>
      <li><strong>Objection (Art. 21)</strong> — email <a href="mailto:{operator.email}" class="underline">{operator.email}</a>.</li>
      <li><strong>Withdraw consent (Art. 7(3))</strong> — withdrawal counts as an erasure request; email <a href="mailto:{operator.email}" class="underline">{operator.email}</a>.</li>
    </ul>
  </section>

  <!-- §6 Complaint -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">6. How to lodge a complaint (Art. 13(2)(d))</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      If you believe your data is being processed unlawfully, you have the right to lodge a complaint with your national data protection authority:
    </p>
    <ul class="text-sm font-semibold text-brand-text-muted list-disc list-inside space-y-1 leading-relaxed">
      <li><strong>France:</strong> CNIL — <a href="https://www.cnil.fr" class="underline">cnil.fr</a></li>
      <li><strong>Germany:</strong> BfDI — <a href="https://www.bfdi.bund.de" class="underline">bfdi.bund.de</a></li>
      <li><strong>UK:</strong> ICO — <a href="https://ico.org.uk" class="underline">ico.org.uk</a></li>
      <li><strong>Other EU/EEA:</strong> <a href="https://edpb.europa.eu/about-edpb/about-edpb/members_en" class="underline">EDPB member list</a></li>
    </ul>
  </section>

  <!-- §7 Min age -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">7. Minimum age (Art. 8)</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      This platform is intended for users aged <strong>16 and above</strong>. By registering,
      you confirm that you meet this requirement. If you are under 16, parental consent must
      be obtained — contact <a href="mailto:{operator.email}" class="underline">{operator.email}</a>.
    </p>
  </section>

  <!-- §8 Cookies -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">8. Cookies (Art. 13)</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">This platform sets exactly one cookie:</p>
    <div class="overflow-x-auto">
      <table class="w-full text-sm text-left border-collapse">
        <thead class="text-[0.7rem] uppercase tracking-[0.1em] text-brand-text-muted">
          <tr>
            <th class="py-2 pr-4 font-bold">Name</th>
            <th class="py-2 pr-4 font-bold">Flags</th>
            <th class="py-2 pr-4 font-bold">Purpose</th>
            <th class="py-2 font-bold">Duration</th>
          </tr>
        </thead>
        <tbody class="text-brand-text-muted font-semibold">
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4"><code>session</code></td>
            <td class="py-2 pr-4"><code>HttpOnly</code>, <code>Secure</code>, <code>SameSite=Strict</code></td>
            <td class="py-2 pr-4">Authentication — required to stay logged in</td>
            <td class="py-2">30 days</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      No tracking, analytics, or advertising cookies are used. No third-party cookies are set.
    </p>
  </section>

  <!-- §9 Processors -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">9. Data processors (Art. 28)</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      Your email address is transmitted to the SMTP provider configured by the operator to send authentication links:
    </p>
    <div class="overflow-x-auto">
      <table class="w-full text-sm text-left border-collapse">
        <thead class="text-[0.7rem] uppercase tracking-[0.1em] text-brand-text-muted">
          <tr>
            <th class="py-2 pr-4 font-bold">Processor</th>
            <th class="py-2 pr-4 font-bold">Role</th>
            <th class="py-2 font-bold">Data sent</th>
          </tr>
        </thead>
        <tbody class="text-brand-text-muted font-semibold">
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{operator.smtp}</td>
            <td class="py-2 pr-4">Transactional SMTP relay</td>
            <td class="py-2">Your email address, authentication link</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      All other data (username, submissions, votes, scores, session, logs, backups) is stored on
      the operator's own infrastructure and is not shared with any third party.
    </p>
  </section>

  <!-- §10 Backups -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">10. Backup disclosure</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      Database backups are retained for 7 days for disaster recovery. If you request erasure,
      your data is deleted from the live database immediately, but may persist in backups for up
      to 7 days. This is permitted under GDPR Art. 17(3)(b) (legitimate interest — incident recovery).
    </p>
  </section>

  <div class="pt-2 pb-4">
    <a href="/" class="text-sm font-bold underline hover:text-brand-text-muted transition-colors">← Back to home</a>
  </div>
</article>
