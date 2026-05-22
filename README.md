# anefs
Anime name extractor from subs
PoC to extract names from anime subs (ass) using rules, regex, weights, etc
Extract character names from `.ass` subtitle files using linguistic signals — no ML, no NLP libraries,
no LLMs. Just Go, regex, and a carefully weighted scoring system.
 
A user on reddit was trying to use llm to extract names from subs - this is just a proof of concept on how 
to do it programmatically.

Used this as test subs.ass file https://pastebin.com/9Wmm7Fwq

Results:

```
Satuki               score=447 count=36
Renako               score=269 count=24
Ajisai               score=221 count=19
Mai                  score=185 count=25
Amaori               score=183 count=16
Hasegawa             score=42  count=3
Kaho                 score=40  count=3
Hirano               score=20  count=1
Koto                 score=20  count=1
```
