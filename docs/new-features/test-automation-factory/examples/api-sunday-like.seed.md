# Seed: API test, Sunday-like (TypeScript)

Level: `api`. Blueprint: `api-sunday-like`. Replace `/orders` and the schema with your endpoint.

```ts
// src/adapters/http.adapter.ts — the interface lives in the consumer layer; this is the real implementation
export interface HttpAdapter {
  post(path: string, body: unknown, options: { timeoutMs: number }): Promise<HttpResponse>;
}
export interface HttpResponse { status: number; durationMs: number; body: unknown }
```

```ts
// src/clients/orders/orders-api.client.ts
import { BaseApiClient } from '../../domain/base-api-client';

const REQUEST_TIMEOUT_MS = 2000;

export class OrdersApiClient extends BaseApiClient {
  createOrder(order: NewOrder) {
    return this.post('/orders', order, { timeoutMs: REQUEST_TIMEOUT_MS });
  }
}
```

```ts
// src/clients/orders/order-response.schema.ts
import { z } from 'zod';
export const orderResponseSchema = z.object({ id: z.string(), status: z.literal('CREATED') });
```

```ts
// tests/api/orders-create.spec.ts
/**
 * @issue EXAMPLE-101
 * @ac AC1: a valid order is created and returns 201
 * @exemplar
 */
test('creates an order and returns 201', async () => {
  // Arrange
  const order = orderFactory.build();
  // Act
  const response = await ordersClient.createOrder(order);
  // Assert
  expect(response).toHaveStatus(201);
  expect(response).toRespondWithin(2000);
  expect(response.validateSchema(orderResponseSchema)).toBe(true);
});
```

## What to notice
- The test names what to call and what to expect. The client owns how.
- The timeout is a named constant. The schema is checked on every response. No `any`, no retry loop.
- The factory and the model live in their own files (`order.factory.ts`, `order.model.ts`).
