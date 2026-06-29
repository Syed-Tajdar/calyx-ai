import { createScorecardSchema, updateScorecardSchema } from './scorecards.schemas';
import { parseZod } from '../common/zod-parse';

describe('scorecards.schemas', () => {
  describe('createScorecardSchema', () => {
    it('accepts valid payload', () => {
      const input = {
        name: 'Medicare QA',
        vertical: 'medicare',
        rules: [{ id: 'greeting', weight: 0.5, question: 'Did agent greet caller?' }],
      };
      expect(parseZod(createScorecardSchema, input)).toEqual({
        ...input,
        is_active: true,
      });
    });

    it('rejects empty name with 422', () => {
      expect(() =>
        parseZod(createScorecardSchema, {
          name: '',
          vertical: 'medicare',
          rules: [{ id: 'greeting', weight: 0.5, question: 'Q?' }],
        }),
      ).toThrow();
    });

    it('rejects empty rules array', () => {
      expect(() =>
        parseZod(createScorecardSchema, {
          name: 'Test',
          vertical: 'medicare',
          rules: [],
        }),
      ).toThrow();
    });

    it('rejects weight outside 0-1', () => {
      expect(() =>
        parseZod(createScorecardSchema, {
          name: 'Test',
          vertical: 'medicare',
          rules: [{ id: 'greeting', weight: 1.5, question: 'Q?' }],
        }),
      ).toThrow();
    });
  });

  describe('updateScorecardSchema', () => {
    it('accepts partial update', () => {
      const result = parseZod(updateScorecardSchema, { name: 'Updated' });
      expect(result).toEqual({ name: 'Updated' });
    });
  });
});
