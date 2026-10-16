# Sources and API verification

Specification research date: October 15, 2026. Implementation fetched the following
documentation without paid inference. Documentation can change; recheck before a
live alpha integration.

- [Client tools](https://openrouter.ai/docs/guides/features/tool-calling)
- [Chat](https://openrouter.ai/docs/api/api-reference/chat/create-a-chat-completion)
- [Models](https://openrouter.ai/docs/guides/overview/models)
- [Model listing](https://openrouter.ai/docs/api/api-reference/models/list-all-models-and-their-properties)
- [Jev](https://openrouter.ai/docs/guides/community/jev)
- [Decisions reference](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request)
- [Decisions tutorial](https://openrouter.ai/docs/guides/community/jev-tutorial)
- [Score](https://docs.typesafe.ai/primitives/score)
- [Confidence](https://docs.typesafe.ai/confidence)
- [Routing](https://openrouter.ai/docs/guides/routing/provider-selection)
- [Errors](https://openrouter.ai/docs/api_reference/errors-and-debugging)
- [Judge limitations](https://arxiv.org/abs/2306.05685)
- [Kamski walkthrough](https://www.powerpyx.com/detroit-become-human-meet-kamski-walkthrough-100/)
- [Encounter cross-check](https://www.shacknews.com/article/105291/meet-kamski---detroit-become-human)
- [HTTP](https://pkg.go.dev/net/http), [HTML escaping](https://pkg.go.dev/html/template)

Fetched Decisions documentation confirms model/state/questions and four score
questions, score/probabilities/confidence/legend responses and origin-relative
/api/alpha/decisions. Its examples currently use 1.13, not the specification's
1.14; this project does not assert 1.14 availability without discovery and a live
response. No moving latest alias or automatic substitution.
