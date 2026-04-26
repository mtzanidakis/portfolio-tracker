export function Icon({ name, size = 16 }) {
  const common = {
    width: size, height: size, viewBox: '0 0 24 24',
    fill: 'none', stroke: 'currentColor',
    strokeWidth: 1.8, strokeLinecap: 'round', strokeLinejoin: 'round',
  };
  const paths = {
    chart:     <><path d="M3 3v18h18" /><path d="M7 14l4-5 3 3 5-7" /></>,
    pie:       <><path d="M21 12a9 9 0 1 1-9-9v9z" /><path d="M12 3a9 9 0 0 1 9 9h-9z" /></>,
    activity:  <><path d="M3 12h4l3-9 4 18 3-9h4" /></>,
    wallet:    <><path d="M20 7H5a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h15a1 1 0 0 0 1-1V8a1 1 0 0 0-1-1Z" /><path d="M16 13h.01" /><path d="M3 9V6a2 2 0 0 1 2-2h11" /></>,
    coins:     <><circle cx="8" cy="8" r="5" /><path d="M16 15.5A5 5 0 1 1 11 11" /><path d="M6 6h4" /><path d="M14 13h4" /></>,
    trash:     <><path d="M3 6h18" /><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></>,
    edit:      <><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" /><path d="M18.5 2.5a2.1 2.1 0 0 1 3 3L12 15l-4 1 1-4Z" /></>,
    plus:      <><path d="M12 5v14M5 12h14" /></>,
    moon:      <><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" /></>,
    sun:       <><circle cx="12" cy="12" r="4" /><path d="M12 3v2M12 19v2M5 12H3M21 12h-2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M5.6 18.4 7 17M17 7l1.4-1.4" /></>,
    eye:       <><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" /></>,
    eyeOff:    <><path d="M9.9 4.24A10 10 0 0 1 12 4c6.5 0 10 7 10 7a13.2 13.2 0 0 1-1.67 2.68" /><path d="M6.61 6.61A13.5 13.5 0 0 0 2 12s3.5 7 10 7a10 10 0 0 0 5.39-1.61" /><path d="m2 2 20 20" /><path d="M9.88 9.88a3 3 0 1 0 4.24 4.24" /></>,
    search:    <><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></>,
    arrowUp:   <><path d="M12 19V5M5 12l7-7 7 7" /></>,
    arrowDown: <><path d="M12 5v14M5 12l7 7 7-7" /></>,
    close:     <><path d="M18 6 6 18M6 6l12 12" /></>,
    more:      <><circle cx="12" cy="12" r="1.5" /><circle cx="5" cy="12" r="1.5" /><circle cx="19" cy="12" r="1.5" /></>,
    check:     <><path d="M20 6 9 17l-5-5" /></>,
    menu:      <><path d="M3 6h18M3 12h18M3 18h18" /></>,
    swap:      <><path d="M7 10V3l-4 4 4 4" /><path d="M17 14v7l4-4-4-4" /><path d="M3 7h14" /><path d="M21 17H7" /></>,
    github:    <path fill="currentColor" stroke="none" d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.92.58.11.79-.25.79-.56 0-.27-.01-1-.02-1.96-3.2.7-3.88-1.54-3.88-1.54-.52-1.32-1.27-1.67-1.27-1.67-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.02 1.75 2.68 1.25 3.34.96.1-.74.4-1.25.72-1.54-2.55-.29-5.24-1.28-5.24-5.69 0-1.26.45-2.29 1.18-3.1-.12-.29-.51-1.46.11-3.04 0 0 .96-.31 3.16 1.18a10.95 10.95 0 0 1 5.76 0c2.2-1.49 3.16-1.18 3.16-1.18.62 1.58.23 2.75.11 3.04.74.81 1.18 1.84 1.18 3.1 0 4.42-2.69 5.39-5.26 5.68.41.36.78 1.06.78 2.14 0 1.55-.01 2.8-.01 3.18 0 .31.21.68.8.56C20.71 21.39 24 17.07 24 12 24 5.65 18.85.5 12 .5z" />,
  };
  return <svg {...common}>{paths[name] || null}</svg>;
}

// Brand mark — three ascending bars + a faint trend line. The rounded
// square background lives in CSS (.brand-mark) so the bars can paint
// in currentColor and inherit the palette via the wrapper's color rule.
export function BrandMark({ size = 20 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none" aria-hidden="true">
      <rect x="11"   y="33" width="11" height="17" rx="2" fill="currentColor" opacity="0.55" />
      <rect x="26.5" y="25" width="11" height="25" rx="2" fill="currentColor" opacity="0.78" />
      <rect x="42"   y="14" width="11" height="36" rx="2" fill="currentColor" />
      <path d="M11 22l14-8 14 4 8-6"
        stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" opacity="0.4" />
    </svg>
  );
}
