# Contributing

## Security

If you think you have found a **security issue**, please do not mention it in this repository.
Instead, send an email to `security@thegeeklab.de` with as many details as possible so it can be handled confidential.

## Bug Reports and Feature Requests

If you have found a **bug** or have a **feature request** please use the search first in case a similar issue already exists.
If not, please create an issue in this repository

## Code

If you would like to fix a bug or implement a feature, please fork the repository and create a Pull Request.

Before you start any Pull Request, it is recommended that you create an issue to discuss first if you have any
doubts about requirement or implementation. That way you can be sure that the maintainer(s) agree on what to change and how,
and you can hopefully get a quick merge afterwards.

Pull Requests can only be merged once all status checks are green.

## Do not force push to your Pull Request branch

Please do not force push to your Pull Requests branch after you have created your Pull Request, as doing so makes it harder for us to review your work.
Pull Requests will always be squashed by us when we merge your work. Commit as many times as you need in your Pull Request branch.

## Re-requesting a review

Please do not ping your reviewer(s) by mentioning them in a new comment. Instead, use the re-request review functionality.
Read more about this in the [GitHub docs, Re-requesting a review](https://docs.github.com/en/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/incorporating-feedback-in-your-pull-request#re-requesting-a-review).

## Translating

The strings are translated using [Weblate](https://weblate.org/en/). Follow [these instructions](https://hosted.weblate.org/engage/renovate-operator/) if you would like to contribute.

Please _do not_ send merge requests or patches modifying the translations. Use Weblate instead - it applies a series of fixes and suggestions, plus it keeps track of modifications and fuzzy translations. Applying translations manually skips all the fixes and checks, and overrides the fuzzy state of strings.

Note that you cannot change the English strings on Weblate. If you have any suggestions on how to improve them, open an issue or merge request like you would if you were making code changes. This way the changes can be reviewed before the source strings on Weblate are changed.
