module.exports = {
  plugins: ["prettier"],
  extends: ["plugin:prettier/recommended"],
  rules: {
    "prettier/prettier": ["error", {
      "endOfLine": "auto",
      "singleQuote": true,
      "semi": true,
      "tabWidth": 2,
      "trailingComma": "all",
      "printWidth": 100
    }],
  },
};
