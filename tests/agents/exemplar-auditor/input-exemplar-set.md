# Fixture: a TypeScript project's exemplar set, three months after adoption

Audit this project's exemplars against `shared/contracts/exemplar-contract.md`.

## `.claude/exemplars.yaml`

```yaml
exemplars:
  - test: creates user with valid credentials
    file: src/user/user-service.spec.ts
    language: typescript
    level: unit
    digest: "sha256:aa11bb22cc33dd44"
    demonstrates: >
      Table-driven cases with descriptive names and boundary values on each
      input dimension, one behaviour per case.

  - test: rejects a request over the rate limit
    file: src/api/rate-limit.spec.ts
    language: typescript
    level: api-contract
    digest: "sha256:ee55ff66aa77bb88"
    demonstrates: >
      Asserts the contract the consumer depends on — status, headers and
      retry-after — without asserting how the limiter computes the window.

  - test: user completes checkout
    file: e2e/checkout.feature
    language: typescript
    level: e2e
    digest: "sha256:1122334455667788"
    demonstrates: >
      Site-Centric flows keep the setup noise out of the scenario while the
      business language stays readable to a non-engineer.
```

## `src/user/user-service.spec.ts` (current content)

```typescript
/**
 * @issue PROJ-140
 * @ac AC1: a user can register with a valid email and password
 * @exemplar
 */
describe('UserService.create', () => {
  it.each([
    ['valid email and password', 'a@b.com', 'hunter2xyz', true],
    ['empty email', '', 'hunter2xyz', false],
    ['password at minimum length', 'a@b.com', '12345678', true],
    ['password one below minimum', 'a@b.com', '1234567', false],
  ])('creates user with valid credentials: %s', async (_name, email, password, expected) => {
    const result = await service.create({ email, password });
    expect(result.ok).toBe(expected);
  });
});
```

## `src/api/rate-limit.spec.ts` (current content)

```typescript
/**
 * @issue PROJ-201
 * @exemplar
 */
test('rejects a request over the rate limit', async () => {
  const limiter = new RateLimiter({ max: 5, windowMs: 60_000 });
  jest.spyOn(limiter, 'isOverLimit').mockReturnValue(true);

  const response = await api.get('/things', { limiter });

  expect(response).toBeDefined();
});
```

## `e2e/checkout.feature`

File does not exist. `git log` shows `e2e/checkout.feature` was renamed to
`e2e/purchase-flow.feature` six weeks ago.

## Project test inventory

The suite contains TypeScript tests at unit, integration, api-contract and e2e levels.
