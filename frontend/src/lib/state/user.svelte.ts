class UserState {
  id = $state<string | null>(null);
  username = $state<string | null>(null);
  email = $state<string | null>(null);
  role = $state<'player' | 'admin' | null>(null);

  isAuthenticated = $derived(this.id !== null);
  isAdmin = $derived(this.role === 'admin');

  setFrom(u: {
    id: string;
    username: string;
    email: string;
    role: 'player' | 'admin';
  }) {
    this.id = u.id;
    this.username = u.username;
    this.email = u.email;
    this.role = u.role;
  }

  clear() {
    this.id = null;
    this.username = null;
    this.email = null;
    this.role = null;
  }
}

export const user = new UserState();
