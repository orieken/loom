// A code emitter: the imports below are text it writes, not imports it makes.
export const fileName = (slug: string) => `${slug}.client.ts`;
export const emitGo = () => `package main

import (
	"context"
	"github.com/example/generated/core"
)
`;
export const emitTs = () => `import { Base } from '../../core/base';
`;
