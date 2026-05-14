import React, { useState, useRef, useEffect } from 'react';

interface VerticalSliderProps {
  slides: React.ReactNode[];
  className?: string;
}

export const VerticalSlider: React.FC<VerticalSliderProps> = ({ slides, className }) => {
  const [current, setCurrent] = useState(0);
  // Each slide keeps its own measured height — updated live by ResizeObserver
  // so async content (charts, lazy data) is always captured correctly.
  const [slideHeights, setSlideHeights] = useState<number[]>([]);
  const [dragOffset, setDragOffset] = useState(0);
  const isDragging = useRef(false);
  const touchStartY = useRef<number | null>(null);
  const slideRefs = useRef<(HTMLDivElement | null)[]>([]);
  const viewportRef = useRef<HTMLDivElement>(null);

  // Watch every slide for size changes. Fires after async content (e.g. ApexCharts)
  // finishes rendering, giving us accurate heights for the track calculation.
  useEffect(() => {
    const observers: ResizeObserver[] = [];

    slideRefs.current.forEach((el, i) => {
      if (!el) return;
      const ro = new ResizeObserver(() => {
        const h = el.offsetHeight;
        setSlideHeights(prev => {
          if (prev[i] === h) return prev;
          const next = [...prev];
          next[i] = h;
          return next;
        });
      });
      ro.observe(el);
      observers.push(ro);
    });

    return () => observers.forEach(ro => ro.disconnect());
  }, [slides.length]);

  // Block the page from scrolling while the user drags inside the slider.
  useEffect(() => {
    const el = viewportRef.current;
    if (!el) return;
    const block = (e: TouchEvent) => {
      if (isDragging.current) e.preventDefault();
    };
    el.addEventListener('touchmove', block, { passive: false });
    return () => el.removeEventListener('touchmove', block);
  }, []);

  const goTo = (index: number) => {
    setCurrent(Math.max(0, Math.min(index, slides.length - 1)));
    setDragOffset(0);
    isDragging.current = false;
  };

  const onTouchStart = (e: React.TouchEvent) => {
    touchStartY.current = e.touches[0].clientY;
    isDragging.current = true;
  };

  const onTouchMove = (e: React.TouchEvent) => {
    if (touchStartY.current === null) return;
    const delta = touchStartY.current - e.touches[0].clientY;
    const atStart = current === 0 && delta < 0;
    const atEnd = current === slides.length - 1 && delta > 0;
    setDragOffset(delta * (atStart || atEnd ? 0.15 : 1));
  };

  const onTouchEnd = (e: React.TouchEvent) => {
    if (touchStartY.current === null) return;
    const delta = touchStartY.current - e.changedTouches[0].clientY;
    if (delta > 55 && current < slides.length - 1) {
      goTo(current + 1);
    } else if (delta < -55 && current > 0) {
      goTo(current - 1);
    } else {
      setDragOffset(0);
      isDragging.current = false;
    }
    touchStartY.current = null;
  };

  if (slides.length === 0) return null;

  // Viewport shows only the current slide's natural height.
  const viewportHeight = slideHeights[current];
  // Track offset = sum of all preceding slide heights.
  const trackOffset = slideHeights.slice(0, current).reduce((sum, h) => sum + h, 0);
  const trackY = -trackOffset - dragOffset;

  return (
    <div className={`relative ${className ?? ''}`}>
      <div
        ref={viewportRef}
        className="overflow-hidden w-full transition-[height] duration-300 ease-out"
        style={viewportHeight ? { height: viewportHeight } : undefined}
        onTouchStart={onTouchStart}
        onTouchMove={onTouchMove}
        onTouchEnd={onTouchEnd}
      >
        <div
          style={{
            transform: `translateY(${trackY}px)`,
            transition: isDragging.current
              ? 'none'
              : 'transform 350ms cubic-bezier(0.32, 0.72, 0, 1)',
          }}
        >
          {slides.map((slide, i) => (
            // No height or overflow set here — each slide is its natural size.
            <div key={i} ref={el => { slideRefs.current[i] = el; }}>
              {slide}
            </div>
          ))}
        </div>
      </div>

      {/* Dot indicators */}
      <div className="flex items-center justify-center gap-2 mt-4">
        {slides.map((_, i) => (
          <button
            key={i}
            type="button"
            onClick={() => goTo(i)}
            aria-label={`Slide ${i + 1} of ${slides.length}`}
            className="group flex items-center justify-center focus:outline-none"
          >
            <span
              className={`block transition-all duration-300 rounded-[3px] ${
                i === current
                  ? 'w-6 h-3 bg-indigo-600'
                  : 'w-3 h-3 bg-gray-300 dark:bg-surface-600 group-hover:bg-indigo-300 dark:group-hover:bg-indigo-700'
              }`}
            />
          </button>
        ))}
      </div>
    </div>
  );
};

export default VerticalSlider;
