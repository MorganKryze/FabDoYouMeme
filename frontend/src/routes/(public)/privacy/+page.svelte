<!-- frontend/src/routes/(public)/privacy/+page.svelte -->
<svelte:head>
  <title>Privacy Policy — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6 text-left w-full max-w-prose mx-auto py-8">
  <h1 class="text-3xl font-bold">Privacy Policy</h1>
  <p class="text-sm text-muted-foreground">Last updated: [operator fills in date]</p>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Who we are</h2>
    <p class="text-sm text-muted-foreground leading-relaxed">
      This service is operated by [operator name and contact — fill in before deployment].
      To contact us about privacy matters, email: [contact email].
    </p>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">What data we collect</h2>
    <ul class="text-sm text-muted-foreground leading-relaxed list-disc list-inside space-y-1">
      <li><strong>Email address</strong> — used to send magic links for authentication.</li>
      <li><strong>Username</strong> — displayed in-game and on leaderboards.</li>
      <li><strong>Game history</strong> — your submissions, votes, and scores.</li>
      <li><strong>One session cookie</strong> — functional only, no tracking. 30-day TTL. HttpOnly.</li>
    </ul>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Why we collect it (lawful basis)</h2>
    <ul class="text-sm text-muted-foreground leading-relaxed list-disc list-inside space-y-1">
      <li><strong>Consent (Art. 6(1)(a) GDPR)</strong> — you give explicit consent at registration for account data processing.</li>
      <li><strong>Legitimate interest (Art. 6(1)(f) GDPR)</strong> — operational logs (up to 30 days) kept for security and reliability.</li>
    </ul>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">How long we keep it</h2>
    <ul class="text-sm text-muted-foreground leading-relaxed list-disc list-inside space-y-1">
      <li>Account data (email, username): until you request deletion.</li>
      <li>Game history (rooms, rounds, submissions, votes): 2 years after the game ends.</li>
      <li>Operational logs: up to 30 days.</li>
      <li>Backups: up to 7 days. Deleted user data may persist in backups for up to 7 days after erasure.</li>
    </ul>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Your rights</h2>
    <ul class="text-sm text-muted-foreground leading-relaxed list-disc list-inside space-y-1">
      <li><strong>Access</strong> — view your data at any time from your profile page.</li>
      <li><strong>Portability</strong> — download your data as JSON from your profile page.</li>
      <li><strong>Rectification</strong> — update your username or email from your profile page.</li>
      <li><strong>Erasure</strong> — contact your admin to request deletion. We respond within 30 days.</li>
      <li><strong>Objection</strong> — contact your admin to object to any processing.</li>
      <li><strong>Supervisory authority</strong> — you may lodge a complaint with your local data protection authority.</li>
    </ul>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Minimum age</h2>
    <p class="text-sm text-muted-foreground leading-relaxed">
      This service is intended for users aged <strong>16 and above</strong>. Registration
      requires explicit age affirmation. If you believe a user under 16 is registered,
      contact your admin immediately.
    </p>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Third-party processors</h2>
    <p class="text-sm text-muted-foreground leading-relaxed">
      Magic-link emails are sent via our SMTP provider ([operator: fill in provider name]).
      Your email address is shared with this provider solely for delivery purposes.
      A Data Processing Agreement (DPA) is in place with the provider per GDPR Art. 28.
    </p>
    <p class="text-sm text-muted-foreground leading-relaxed">
      All other data remains on-premises. No analytics, tracking, or advertising services are used.
    </p>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Cookies</h2>
    <p class="text-sm text-muted-foreground leading-relaxed">
      We use exactly one cookie: a session cookie set when you log in via magic link.
      It is <strong>HttpOnly</strong> (not accessible to JavaScript), <strong>Secure</strong>
      (HTTPS only), and <strong>SameSite=Strict</strong> (CSRF protection).
      It expires after 30 days of inactivity. No third-party cookies are set.
    </p>
  </section>

  <div class="pt-4">
    <a href="/" class="text-sm underline hover:text-foreground">← Back to home</a>
  </div>
</div>
