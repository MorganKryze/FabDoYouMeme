export type ToastType = 'success' | 'warning' | 'error';

interface ToastItem {
  id: number;
  message: string;
  type: ToastType;
  /** Duration in ms. 0 = persistent (manual dismiss required). */
  duration: number;
}

export class ToastState {
  #items = $state<ToastItem[]>([]);
  #nextId = 0;

  get items(): ToastItem[] {
    return this.#items;
  }

  show(message: string, type: ToastType = 'success'): void {
    const duration = type === 'error' ? 0 : type === 'warning' ? 5000 : 3000;
    const item: ToastItem = { id: this.#nextId++, message, type, duration };

    // Max 3 visible — drop oldest
    if (this.#items.length >= 3) {
      this.#items = this.#items.slice(1);
    }
    this.#items = [...this.#items, item];

    if (duration > 0) {
      setTimeout(() => this.dismiss(item.id), duration);
    }
  }

  dismiss(id: number): void {
    this.#items = this.#items.filter(t => t.id !== id);
  }
}

export const toast = new ToastState();
