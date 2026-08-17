declare global {
  interface Window {
    Telegram?: {
      WebApp?: {
        ready: () => void;
        expand: () => void;
        initData: string;
        initDataUnsafe: { user?: { id: number } };
        themeParams: Record<string, string>;
        colorScheme: 'light' | 'dark';
        viewportStableHeight?: number;
        onEvent?: (event: string, cb: () => void) => void;
        offEvent?: (event: string, cb: () => void) => void;
      };
    };
  }
}

export {};
