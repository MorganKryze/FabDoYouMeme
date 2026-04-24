<!-- frontend/src/routes/(public)/privacy/+page.svelte -->
<script lang="ts">
  import { env } from '$env/dynamic/public';
  import { reveal } from '$lib/actions/reveal';
  import * as m from '$lib/paraglide/messages';

  const LAST_UPDATED = '2026-04-13';
  const VERSION = '1.0';

  const fallback = (name: string) => `[${name} not set, see docs/self-hosting.md]`;

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
  <title>{m.privacy_page_title()}</title>
  <meta name="description" content={m.privacy_meta_description()} />
</svelte:head>

<article class="mx-auto max-w-3xl w-full flex flex-col gap-8" use:reveal>
  <header class="flex flex-col gap-3 pb-6 border-b-[2.5px] border-brand-border-heavy">
    <h1 class="text-4xl font-bold tracking-tight">{m.privacy_heading()}</h1>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      {m.privacy_intro()}
    </p>
    <p class="text-xs text-brand-text-muted">
      {m.privacy_last_updated_version({ date: LAST_UPDATED, version: VERSION })}
    </p>
  </header>

  {#if missingVars.length > 0}
    <div
      role="alert"
      class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-accent/20 px-5 py-4 text-sm font-semibold leading-relaxed"
    >
      <strong>{m.privacy_alert_heading()}</strong>
      {m.privacy_alert_prefix()}
      {#each missingVars as name, i}<code class="font-mono">{name}</code>{#if i < missingVars.length - 1}, {/if}{/each}.
      {m.privacy_alert_see_docs_prefix()}<code>docs/self-hosting.md → Legal / privacy policy</code>{m.privacy_alert_see_docs_suffix()}
      {m.privacy_alert_not_compliant()}
    </div>
  {/if}

  <!-- §1 Controller identity -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s1_heading()}</h2>
    <ul class="text-sm font-semibold text-brand-text-muted space-y-2 leading-relaxed">
      <li><strong>{m.privacy_s1_controller_label()}</strong> {operator.name}</li>
      <li>
        <strong>{m.privacy_s1_email_label()}</strong>
        <a href="mailto:{operator.email}" class="underline hover:text-brand-text transition-colors">{operator.email}</a>{m.privacy_s1_email_suffix()}
      </li>
      <li><strong>{m.privacy_s1_hosted_label()}</strong> {operator.url}</li>
    </ul>
  </section>

  <!-- §2 Purposes -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s2_heading()}</h2>
    <div class="overflow-x-auto">
      <table class="w-full text-sm text-left border-collapse">
        <thead class="text-[0.7rem] uppercase tracking-[0.1em] text-brand-text-muted">
          <tr>
            <th class="py-2 pr-4 font-bold">{m.privacy_s2_th_purpose()}</th>
            <th class="py-2 pr-4 font-bold">{m.privacy_s2_th_data()}</th>
            <th class="py-2 font-bold">{m.privacy_s2_th_basis()}</th>
          </tr>
        </thead>
        <tbody class="text-brand-text-muted font-semibold">
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s2_row1_purpose()}</td>
            <td class="py-2 pr-4">{m.privacy_s2_row1_data()}</td>
            <td class="py-2">{m.privacy_s2_row1_basis()}</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s2_row2_purpose()}</td>
            <td class="py-2 pr-4">{m.privacy_s2_row2_data()}</td>
            <td class="py-2">{m.privacy_s2_row2_basis()}</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s2_row3_purpose()}</td>
            <td class="py-2 pr-4">{m.privacy_s2_row3_data()}</td>
            <td class="py-2">{m.privacy_s2_row3_basis()}</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s2_row4_purpose()}</td>
            <td class="py-2 pr-4">{m.privacy_s2_row4_data()}</td>
            <td class="py-2">{m.privacy_s2_row4_basis()}</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s2_row5_purpose()}</td>
            <td class="py-2 pr-4">{m.privacy_s2_row5_data()}</td>
            <td class="py-2">{m.privacy_s2_row5_basis()}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <!-- §3 Categories -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s3_heading()}</h2>
    <ul class="text-sm font-semibold text-brand-text-muted list-disc list-inside space-y-1 leading-relaxed">
      <li><strong>{m.privacy_s3_email_label()}</strong>{m.privacy_s3_email_body()}</li>
      <li><strong>{m.privacy_s3_username_label()}</strong>{m.privacy_s3_username_body()}</li>
      <li><strong>{m.privacy_s3_consent_label()}</strong>{m.privacy_s3_consent_body()}</li>
      <li><strong>{m.privacy_s3_submissions_label()}</strong>{m.privacy_s3_submissions_body()}</li>
      <li><strong>{m.privacy_s3_scores_label()}</strong>{m.privacy_s3_scores_body()}</li>
      <li><strong>{m.privacy_s3_session_label()}</strong>{m.privacy_s3_session_body_prefix()}<code>HttpOnly</code>{m.privacy_s3_session_body_suffix()}</li>
    </ul>
  </section>

  <!-- §4 Retention -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s4_heading()}</h2>
    <div class="overflow-x-auto">
      <table class="w-full text-sm text-left border-collapse">
        <thead class="text-[0.7rem] uppercase tracking-[0.1em] text-brand-text-muted">
          <tr>
            <th class="py-2 pr-4 font-bold">{m.privacy_s4_th_data()}</th>
            <th class="py-2 font-bold">{m.privacy_s4_th_retention()}</th>
          </tr>
        </thead>
        <tbody class="text-brand-text-muted font-semibold">
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s4_account()}</td>
            <td class="py-2">{m.privacy_s4_account_retention()}</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s4_game()}</td>
            <td class="py-2">{m.privacy_s4_game_retention()}</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s4_session()}</td>
            <td class="py-2">{m.privacy_s4_session_retention()}</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s4_tokens()}</td>
            <td class="py-2">{m.privacy_s4_tokens_retention()}</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s4_logs()}</td>
            <td class="py-2">{m.privacy_s4_logs_retention()}</td>
          </tr>
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{m.privacy_s4_backups()}</td>
            <td class="py-2">{m.privacy_s4_backups_retention()}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <!-- §5 Rights -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s5_heading()}</h2>
    <ul class="text-sm font-semibold text-brand-text-muted space-y-2 leading-relaxed">
      <li><strong>{m.privacy_s5_access_label()}</strong>{m.privacy_s5_access_body()}</li>
      <li><strong>{m.privacy_s5_portability_label()}</strong>{m.privacy_s5_portability_body()}</li>
      <li><strong>{m.privacy_s5_rectification_label()}</strong>{m.privacy_s5_rectification_body()}</li>
      <li><strong>{m.privacy_s5_erasure_label()}</strong>{m.privacy_s5_erasure_prefix()}<a href="mailto:{operator.email}" class="underline">{operator.email}</a>{m.privacy_s5_erasure_suffix()}</li>
      <li><strong>{m.privacy_s5_objection_label()}</strong>{m.privacy_s5_objection_prefix()}<a href="mailto:{operator.email}" class="underline">{operator.email}</a>{m.privacy_s5_objection_suffix()}</li>
      <li><strong>{m.privacy_s5_withdraw_label()}</strong>{m.privacy_s5_withdraw_prefix()}<a href="mailto:{operator.email}" class="underline">{operator.email}</a>{m.privacy_s5_withdraw_suffix()}</li>
    </ul>
  </section>

  <!-- §6 Complaint -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s6_heading()}</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      {m.privacy_s6_intro()}
    </p>
    <ul class="text-sm font-semibold text-brand-text-muted list-disc list-inside space-y-1 leading-relaxed">
      <li><strong>{m.privacy_s6_france_label()}</strong> CNIL, <a href="https://www.cnil.fr" class="underline">cnil.fr</a></li>
      <li><strong>{m.privacy_s6_germany_label()}</strong> BfDI, <a href="https://www.bfdi.bund.de" class="underline">bfdi.bund.de</a></li>
      <li><strong>{m.privacy_s6_uk_label()}</strong> ICO, <a href="https://ico.org.uk" class="underline">ico.org.uk</a></li>
      <li><strong>{m.privacy_s6_other_label()}</strong> <a href="https://edpb.europa.eu/about-edpb/about-edpb/members_en" class="underline">{m.privacy_s6_other_link_text()}</a></li>
    </ul>
  </section>

  <!-- §7 Min age -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s7_heading()}</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      {m.privacy_s7_body_prefix()}<strong>{m.privacy_s7_body_strong()}</strong>{m.privacy_s7_body_suffix()}<a href="mailto:{operator.email}" class="underline">{operator.email}</a>{m.privacy_s7_body_end()}
    </p>
  </section>

  <!-- §8 Cookies -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s8_heading()}</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">{m.privacy_s8_intro()}</p>
    <div class="overflow-x-auto">
      <table class="w-full text-sm text-left border-collapse">
        <thead class="text-[0.7rem] uppercase tracking-[0.1em] text-brand-text-muted">
          <tr>
            <th class="py-2 pr-4 font-bold">{m.privacy_s8_th_name()}</th>
            <th class="py-2 pr-4 font-bold">{m.privacy_s8_th_flags()}</th>
            <th class="py-2 pr-4 font-bold">{m.privacy_s8_th_purpose()}</th>
            <th class="py-2 font-bold">{m.privacy_s8_th_duration()}</th>
          </tr>
        </thead>
        <tbody class="text-brand-text-muted font-semibold">
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4"><code>session</code></td>
            <td class="py-2 pr-4"><code>HttpOnly</code>, <code>Secure</code>, <code>SameSite=Strict</code></td>
            <td class="py-2 pr-4">{m.privacy_s8_session_purpose()}</td>
            <td class="py-2">{m.privacy_s8_session_duration()}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      {m.privacy_s8_outro()}
    </p>
  </section>

  <!-- §9 Processors -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s9_heading()}</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      {m.privacy_s9_intro()}
    </p>
    <div class="overflow-x-auto">
      <table class="w-full text-sm text-left border-collapse">
        <thead class="text-[0.7rem] uppercase tracking-[0.1em] text-brand-text-muted">
          <tr>
            <th class="py-2 pr-4 font-bold">{m.privacy_s9_th_processor()}</th>
            <th class="py-2 pr-4 font-bold">{m.privacy_s9_th_role()}</th>
            <th class="py-2 font-bold">{m.privacy_s9_th_data()}</th>
          </tr>
        </thead>
        <tbody class="text-brand-text-muted font-semibold">
          <tr class="border-t border-brand-border">
            <td class="py-2 pr-4">{operator.smtp}</td>
            <td class="py-2 pr-4">{m.privacy_s9_smtp_role()}</td>
            <td class="py-2">{m.privacy_s9_smtp_data()}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      {m.privacy_s9_outro()}
    </p>
  </section>

  <!-- §10 Backups -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s10_heading()}</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed">
      {m.privacy_s10_body()}
    </p>
  </section>

  <!-- §11 Groups -->
  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-bold">{m.privacy_s11_heading()}</h2>
    <p class="text-sm font-semibold text-brand-text-muted leading-relaxed whitespace-pre-line">
      {m.privacy_s11_body()}
    </p>
  </section>

  <div class="pt-2 pb-4">
    <a href="/" class="text-sm font-bold underline hover:text-brand-text-muted transition-colors">{m.privacy_back_home()}</a>
  </div>
</article>
