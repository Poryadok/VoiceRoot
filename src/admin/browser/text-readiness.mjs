export function bodyTextIncludesExpression(value) {
  // Page.navigate can acknowledge a new document before its body is parsed.
  return `document.body !== null && document.body.innerText.includes(${JSON.stringify(value)})`;
}
