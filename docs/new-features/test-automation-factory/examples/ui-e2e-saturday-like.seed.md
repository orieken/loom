# Seed: UI end-to-end test, Saturday-like (TypeScript, Playwright, Gherkin)

Level: `ui-e2e`. Blueprint: `ui-e2e-saturday-like`. Replace the login page with a real page.

```gherkin
# features/authentication.feature
@issue-EXAMPLE-201 @exemplar
Feature: Authentication

  Scenario: A registered user signs in and sees their dashboard
    Given a registered user
    When the user signs in with valid credentials
    Then the dashboard greets the user by name
```

```ts
// src/sites/storefront/pages/login.page.ts — a page delegates to elements; it never calls the driver
export class LoginPage extends BasePage {
  readonly usernameInput = this.element('username-input');
  readonly passwordInput = this.element('password-input');
  readonly submitButton = this.element('submit-button');
}
```

```ts
// src/sites/storefront/flows/sign-in.flow.ts — a flow chains pages into a business goal
export class SignInFlow extends BaseFlow {
  async signIn(user: TestUser): Promise<void> {
    await this.loginPage.usernameInput.fill(user.username);
    await this.loginPage.passwordInput.fill(user.password);
    await this.loginPage.submitButton.click();
  }
}
```

```ts
// steps/authentication.steps.ts — the step shows the user action; setup noise stays in the flow and the factory
When('the user signs in with valid credentials', async function () {
  await this.signInFlow.signIn(this.registeredUser);
});
```

## What to notice
- The scenario name is the acceptance criterion. No AC annotation is needed on Gherkin.
- Credentials come from a user factory that reads environment variables. No secret appears in the files.
- Telemetry spans come from the adapter, not from the page or flow.
- There is no fixed sleep. Elements wait for conditions.
