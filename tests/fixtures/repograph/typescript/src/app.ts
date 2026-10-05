import { clamp } from './util';
import format from '@lib/format';
import type { Config } from "./types";
import React from 'react';
import { ghost } from '@ghost/thing';
import './missing';
// import nothing from './commented-out';
/* import nothing from './block-commented-out'; */
export * from './lib/format';
const helpers = require('./helpers.js');

export async function load(name: string) {
  return import(`./plugins/${name}`);
}
