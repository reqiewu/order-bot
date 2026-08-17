import { useEffect, useRef, useState, type ComponentType, type CSSProperties } from 'react';

const cache = new Map<string, Promise<unknown>>();

type LottieProps = {
  animationData: unknown;
  loop?: boolean;
  autoplay?: boolean;
  style?: CSSProperties;
};

async function decodeTgs(buf: ArrayBuffer): Promise<unknown> {
  const u8 = new Uint8Array(buf);
  if (u8.length >= 2 && u8[0] === 0x1f && u8[1] === 0x8b) {
    const ds = new DecompressionStream('gzip');
    const out = await new Response(new Blob([buf]).stream().pipeThrough(ds)).arrayBuffer();
    return JSON.parse(new TextDecoder().decode(out));
  }
  return JSON.parse(new TextDecoder().decode(u8));
}

function loadTgs(url: string): Promise<unknown> {
  const hit = cache.get(url);
  if (hit) return hit;
  const p = (async () => {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`tgs ${res.status}`);
    return decodeTgs(await res.arrayBuffer());
  })();
  cache.set(url, p);
  p.catch(() => cache.delete(url));
  return p;
}

let lottiePromise: Promise<ComponentType<LottieProps>> | null = null;

function loadLottie(): Promise<ComponentType<LottieProps>> {
  if (!lottiePromise) {
    lottiePromise = import('lottie-react').then((m) => m.default as ComponentType<LottieProps>);
  }
  return lottiePromise;
}

type Props = {
  src: string;
  fallbackSrc?: string;
  className?: string;
};

export function TgsPlayer({ src, fallbackSrc, className }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  const [data, setData] = useState<unknown>(null);
  const [fail, setFail] = useState(false);
  const [Lottie, setLottie] = useState<ComponentType<LottieProps> | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const io = new IntersectionObserver(
      ([e]) => {
        if (e.isIntersecting) setVisible(true);
      },
      { rootMargin: '120px' },
    );
    io.observe(el);
    return () => io.disconnect();
  }, []);

  useEffect(() => {
    setData(null);
    setFail(false);
  }, [src]);

  useEffect(() => {
    if (!visible || !src) return;
    let cancelled = false;
    void loadLottie().then((Comp) => {
      if (!cancelled) setLottie(() => Comp);
    });
    loadTgs(src)
      .then((d) => {
        if (!cancelled) setData(d);
      })
      .catch(() => {
        if (!cancelled) setFail(true);
      });
    return () => {
      cancelled = true;
    };
  }, [visible, src]);

  if (fail && fallbackSrc) {
    return <img className={className} src={fallbackSrc} alt="" />;
  }

  return (
    <div ref={ref} className={className} aria-hidden>
      {data && Lottie ? (
        <Lottie animationData={data} loop autoplay style={{ width: '100%', height: '100%' }} />
      ) : null}
    </div>
  );
}
