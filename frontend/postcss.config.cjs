// Tailwind rejects `@layer base|components|utilities` unless the same file has the
// matching `@tailwind` directive. The vendored Oat sheets and layout.css use native
// cascade layers, so hide their `@layer` at-rules from Tailwind and restore them after.
const isNative = file => /[\\/]vendor[\\/]oat[\\/]|[\\/]layout\.css$/.test(file || '')
const rename = (from, to) => root => {
  if (!isNative(root.source && root.source.input && root.source.input.file)) return
  root.walkAtRules(from, rule => { rule.name = to })
}

module.exports = {
  plugins: [
    { postcssPlugin: 'hide-native-layers', Once: rename('layer', 'native-layer') },
    require('tailwindcss/nesting'),
    require('tailwindcss')({ config: './tailwind.config.cjs' }),
    { postcssPlugin: 'restore-native-layers', Once: rename('native-layer', 'layer') },
    require('autoprefixer'),
  ],
}
