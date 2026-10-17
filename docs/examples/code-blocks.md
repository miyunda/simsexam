# Subject: code-block-demo

## Meta
- slug: code-block-demo
- title: Code Block Demo
- duration_minutes: 5
- question_count: 1
- access_level: free
- status: published
- version: 2026-10-17

---

## Question
key: policy-001
type: single

The following policies are attached to the same identity. Assume no other policies apply. Which statement is correct?

Policy 1:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "iam:Get*",
        "iam:List*",
        "kms:List*",
        "ec2:*",
        "ds:*",
        "logs:Get*",
        "logs:Describe*"
      ],
      "Resource": "*"
    }
  ]
}
```

Policy 2:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Deny",
      "Action": "ds:Delete*",
      "Resource": "*"
    }
  ]
}
```

- [x] The explicit deny applies to actions matching `ds:Delete*`.
- [ ] Policy 1 overrides Policy 2 because it allows `ds:*`.
- [ ] All Directory Service actions are denied.
- [ ] All EC2 actions are denied.

### Explanation

The explicit deny in Policy 2 applies to actions matching:

```text
ds:Delete*
```
