import test from 'node:test';
import {assertApprovedFixtureWardrobe} from './support/approved-fixture-check.mjs';
test('the actual isolated approved fixture performs local wardrobe changes and simulated failure never changes recorded wear',assertApprovedFixtureWardrobe);
