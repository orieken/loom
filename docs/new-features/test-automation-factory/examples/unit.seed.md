# Seed: Unit test (TypeScript, Vitest)

Level: `unit`. Blueprint: `unit`. Replace the validator with a real unit.

```ts
// tests/unit/order-total.spec.ts
/**
 * @issue EXAMPLE-301
 * @ac AC1: an order with a discount code totals the discounted price
 * @exemplar
 */
describe('calculateOrderTotal', () => {
  it('applies the percentage discount to the subtotal', () => {
    // Arrange
    const order = orderFactory.build({ subtotalCents: 10_000, discountPercent: 10 });
    // Act
    const total = calculateOrderTotal(order);
    // Assert
    expect(total).toBe(9_000);
  });

  it('returns the subtotal when no discount code is present', () => {
    const order = orderFactory.build({ subtotalCents: 10_000, discountPercent: 0 });
    expect(calculateOrderTotal(order)).toBe(10_000);
  });
});
```

## What to notice
- Arrange, Act, Assert, one concept per test.
- The test name states behavior, not a method name.
- The clock, network, and filesystem do not appear. When a unit needs one, it receives an interface and the test passes a fake.
