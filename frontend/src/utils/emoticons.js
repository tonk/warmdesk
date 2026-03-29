// Text emoticons → emoji replacements.
// Sorted longest-first so greedy matching prefers e.g. ":-)" over ":)".
export const EMOTICONS = [
  ["O:-)",  "😇"],
  [":'-)",  "😂"],
  [":'-(", "😢"],
  [">:-(", "😠"],
  [":-)",  "😊"],
  [":-D",  "😄"],
  [":-P",  "😛"],
  [":-p",  "😛"],
  [":-O",  "😮"],
  [":-o",  "😮"],
  [":-*",  "😘"],
  [":-|",  "😐"],
  [":-/",  "😕"],
  [":-X",  "🤐"],
  [":-x",  "🤐"],
  [";-)",  "😉"],
  ["B-)",  "😎"],
  ["O:)",  "😇"],
  ["</3",  "💔"],
  [">:(",  "😠"],
  [":'(",  "😢"],
  [":)",   "😊"],
  [":(",   "😢"],
  [":D",   "😄"],
  [":P",   "😛"],
  [":p",   "😛"],
  [":O",   "😮"],
  [":o",   "😮"],
  [":*",   "😘"],
  [":|",   "😐"],
  [":/",   "😕"],
  [":X",   "🤐"],
  [":x",   "🤐"],
  [";)",   "😉"],
  ["<3",   "❤️"],
]

/**
 * Check if `text` ends with a known emoticon that is either at the start of
 * the string or preceded by a whitespace character.
 * Returns { pattern, emoji } if found, otherwise null.
 */
export function detectEmoticon(text) {
  for (const [pattern, emoji] of EMOTICONS) {
    if (!text.endsWith(pattern)) continue
    const before = text[text.length - pattern.length - 1]
    if (before !== undefined && before !== ' ' && before !== '\n' && before !== '\t') continue
    return { pattern, emoji }
  }
  return null
}
