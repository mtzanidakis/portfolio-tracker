import { options } from 'preact';

// Preact 11 core no longer appends "px" to numeric inline-style values
// (only preact/compat does), so `style={{ width: 32 }}` would emit an
// invalid declaration the browser drops. The components are written
// React-style throughout, so restore the old behaviour in one place
// instead of rewriting every style object. The unitless-property regex
// is the one preact/compat uses (lineHeight, opacity, flex, zIndex, …).
const IS_NON_DIMENSIONAL =
  /^(-|f[lo].*[^se]$|g.{5,}[^ps]$|z|o[pr]|(W.{5})?[lL]i.*(t|mp)$|an|(bo|s).{4}Im|sca|m.{6}[ds]|ta|c.*[st]$|wido|ini)/;

const prevVnode = options.vnode;
options.vnode = (vnode) => {
  const style = typeof vnode.type === 'string' && vnode.props.style;
  if (style && typeof style === 'object') {
    let cloned;
    for (const key in style) {
      if (typeof style[key] === 'number' && !IS_NON_DIMENSIONAL.test(key)) {
        // Clone before mutating: style objects are often shared
        // (e.g. AssetLogo's `base`) or module-level constants.
        if (!cloned) cloned = vnode.props.style = { ...style };
        cloned[key] += 'px';
      }
    }
  }
  if (prevVnode) prevVnode(vnode);
};
