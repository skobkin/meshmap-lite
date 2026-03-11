export default {
  extends: ['stylelint-config-standard'],
  ignoreFiles: ['coverage/**', 'dist/**', 'node_modules/**'],
  rules: {
    'no-descending-specificity': null,
    'property-no-vendor-prefix': null
  }
}
