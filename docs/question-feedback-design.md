# Question Feedback Design Note

This note outlines a small product and technical design for reporting question quality issues from the learner side.

## Goal

Allow learners to report problematic questions when they are confident that a question, answer key, explanation, or wording is wrong or unclear.

The purpose is not to build a generic support inbox. The purpose is to improve question-bank quality.

## Why This Matters

The question bank is a core asset of `simsexam`. When a question is wrong, ambiguous, outdated, or poorly formatted, the learner experience degrades immediately.

Learners are often the first people to discover these problems, especially when:

- they know the material well
- they compare the result with the official answer and still believe the question is flawed
- the same issue appears repeatedly across multiple sessions

A lightweight feedback channel makes those issues visible to administrators before they become widespread trust problems.

## Product Positioning

This feature should be presented as:

- question quality feedback

It should not be positioned as:

- a general support channel
- a chat with administrators
- a direct edit request that bypasses review

## Recommended Scope

### First Version

The first version should stay small and structured.

Learners can:

- report an issue for a specific question
- choose a feedback type
- optionally leave a short comment

The system should automatically attach context so the learner does not need to explain which question they mean.

Administrators can:

- review submitted feedback
- filter feedback by subject or status
- see repeated reports for the same question
- jump from feedback to the question editor
- mark the issue as resolved or dismissed

### Not In Scope For First Version

- free-form support threads
- email replies
- file attachments
- moderation workflows with multiple assignees
- automatic notifications to learners
- direct public discussion between learners

## Recommended Entry Points

### During The Exam

Provide a low-disruption entry point on the question page, for example:

- `Report an issue`

This should not interrupt answer submission or navigation.

### On The Result Page

This should be the primary entry point.

By the time learners see the result page, they have already seen:

- their answer
- the correct answer
- the explanation

That makes question-quality feedback more deliberate and less impulsive.

## Feedback Types

Use a short, fixed list in the first version:

- `incorrect_answer`
- `ambiguous_wording`
- `outdated_content`
- `typo_or_formatting`
- `other`

Why a fixed list matters:

- administrators can filter and prioritize more easily
- reporting is faster for learners
- analytics later become simpler

## Context That Should Be Captured Automatically

Every feedback record should include enough context for an administrator to understand what the learner saw.

Recommended automatic fields:

- `subject_id`
- `question_id`
- `question_set_id`
- `exam_id` when available
- `exam_question_id` when available
- `user_id` when available
- `feedback_type`
- `comment`
- `question_snapshot_json`
- `answer_snapshot_json`
- `created_at`

### Question Snapshot

The snapshot should capture the content state the learner actually saw, for example:

- question stem
- explanation
- option list in displayed order
- which options were marked correct in that version

This matters because:

- the question may later be edited
- administrators need to review the reported content as seen at the time

### Answer Snapshot

If the learner reports from an exam session, include:

- selected option IDs
- selected option texts
- displayed option order
- whether the learner's submitted answer was judged correct

This helps distinguish:

- a genuinely flawed question
- a learner disagreement that comes from a misunderstanding

## Data Model Direction

### Single-Table First Version

A single table is enough for the first version.

Suggested table:

- `question_feedback`

Suggested columns:

- `id`
- `subject_id`
- `question_id`
- `question_set_id`
- `exam_id`
- `exam_question_id`
- `user_id`
- `feedback_type`
- `comment`
- `status`
- `resolution_note`
- `question_snapshot_json`
- `answer_snapshot_json`
- `created_at`
- `resolved_at`
- `resolved_by_user_id`

### Status Values

Keep the first version simple:

- `open`
- `resolved`
- `dismissed`

Meaning:

- `open`: needs administrator review
- `resolved`: a real issue was addressed
- `dismissed`: no action was needed or the report was not valid

### Why Not A More Complex Model Yet

A multi-table ticket system can come later if needed.

For now, one record per learner report is enough because the primary value is:

- issue discovery
- issue triage
- question-level aggregation

## Admin Workflow

### Feedback List

The first version should present feedback as a question-quality work queue, not as a support inbox.

Provide an admin page with:

- status filter
- subject filter
- feedback type filter
- sort by newest
- per-row report count for the same question
- a direct link to the feedback detail page

Each row should show:

- subject title
- question key or stem excerpt
- feedback type
- current status
- created time
- total reports for the same question

The default list view should focus on `open` feedback so administrators land on unresolved items first.

### Feedback Detail

Each feedback item should have a dedicated detail page.

That page should be split into two sections:

- `Learner Report`
- `Exam Context`

The learner report section should show:

- feedback type
- optional learner comment
- created time
- current status
- resolution note when present

The exam context section should show:

- question stem snapshot
- explanation snapshot
- option list in displayed order
- which options were marked correct in that snapshot
- learner-selected options
- whether the learner answer was judged correct

The detail page should provide only a small set of admin actions:

- `Resolve`
- `Dismiss`
- `Edit Question`

### Question-Level Aggregation

The most valuable admin view is not a raw chronological stream.

Administrators need to see:

- which questions receive repeated reports
- which subjects have the most disputed questions
- which issue types are most common

A future aggregated view should show:

- question key or stem excerpt
- total report count
- last reported time
- most common feedback type
- current status summary

### Question Editing Flow

From a feedback item, the administrator should be able to:

- open the related question
- review the report context
- edit the question if needed
- save a resolution note
- mark the feedback as resolved or dismissed

This keeps feedback as a signal and review surface, while the actual question change still happens through the normal question editor and revision history flow.

Question feedback should never update the question bank automatically.

Feedback is a signal. Editing remains an administrator action with revision history.

## Learner Experience Recommendations

### Keep The Form Short

The learner should not have to fill in a long report.

Recommended fields:

- feedback type selector
- short optional comment textarea
- submit button

Everything else should come from application context.

### Do Not Require The Learner To Explain IDs

The form should never ask for:

- subject name
- question key
- answer key details
- which attempt this came from

Those should already be known by the system.

### Anonymous vs Authenticated Feedback

The long-term direction should favor authenticated learners because their reports are easier to interpret and analyze.

If anonymous feedback is allowed later:

- bind it to a concrete question and, when available, an exam session
- expect higher noise

For the current project stage, the design should assume:

- feedback may exist before full login features are complete
- but authenticated reporting will be more valuable once accounts are added

## Snapshot Strategy

There are two possible approaches:

### Reference-Only

Store only IDs and resolve current content later.

This is simpler, but it loses historical fidelity when questions change.

### Snapshot-Backed

Store IDs and a small JSON snapshot of the question and answer context at submission time.

This is the recommended approach because:

- it preserves what the learner actually saw
- it avoids confusion after question edits
- it supports future auditing

## Analytics Value Later

Even a simple feedback feature can later support useful quality signals, such as:

- questions with high dispute rates
- subjects with repeated ambiguity reports
- changes in report volume after a question revision

This should not drive the first implementation, but the schema should avoid blocking those future analyses.

## Testing Considerations For A Future Implementation

Minimum tests should cover:

- feedback can only be submitted for an existing question
- exam-linked feedback stores the right exam context
- result-page feedback stores displayed option order snapshot
- admin can filter by status and subject
- admin can resolve and dismiss feedback
- question editing remains separate from feedback submission

## Recommended Rollout

### Phase 1

- add `question_feedback`
- add learner-side submission flow
- add basic admin list view
- add resolve and dismiss actions

### Phase 2

- add aggregated question-level reporting view
- add links between feedback and question revisions
- add optional notifications or follow-up workflows

## Recommendation

`simsexam` should eventually include a structured question feedback channel.

The first version should be deliberately small:

- question-specific
- structured
- context-rich
- administrator-reviewed

That gives the project a practical quality-improvement loop without turning the product into a general support system.
