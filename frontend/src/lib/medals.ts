import type { HistoryRoom } from '../routes/(app)/home/+page.server';

export type MedalId = 'welcomed' | 'first-game' | 'first-win' | 'veteran';

export interface Medal {
  id: MedalId;
  name: string;
  icon: string;
  description: string;
  earned: boolean;
}

export function computeMedals(
  user: { created_at: string },
  history: HistoryRoom[],
): Medal[] {
  const plays = history.length;
  const wins = history.some((r) => r.rank === 1);

  return [
    {
      id: 'welcomed',
      name: 'Welcomed',
      icon: '👋',
      description: 'Joined the Maker club',
      earned: true,
    },
    {
      id: 'first-game',
      name: 'First Game',
      icon: '🎮',
      description: 'Played your first round',
      earned: plays >= 1,
    },
    {
      id: 'first-win',
      name: 'First Win',
      icon: '🏆',
      description: 'Topped the leaderboard',
      earned: wins,
    },
    {
      id: 'veteran',
      name: 'Veteran',
      icon: '🎖️',
      description: 'Ten games in the books',
      earned: plays >= 10,
    },
  ];
}

export function formatMakerSince(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleDateString(undefined, { month: 'short', year: 'numeric' });
}
