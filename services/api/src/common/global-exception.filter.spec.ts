import { ArgumentsHost, HttpException, HttpStatus } from '@nestjs/common';
import { ZodError, z } from 'zod';
import { GlobalExceptionFilter } from './global-exception.filter';

function mockHost(): {
  host: ArgumentsHost;
  status: jest.Mock;
  json: jest.Mock;
} {
  const json = jest.fn();
  const status = jest.fn().mockReturnValue({ json });
  const res = { status, getHeader: jest.fn(), setHeader: jest.fn() };
  const req = { url: '/test', headers: {} };
  const host = {
    switchToHttp: () => ({
      getResponse: () => res,
      getRequest: () => req,
    }),
  } as unknown as ArgumentsHost;
  return { host, status, json };
}

describe('GlobalExceptionFilter', () => {
  it('maps ZodError to 400 with field messages', () => {
    const filter = new GlobalExceptionFilter();
    const { host, status, json } = mockHost();
    let err: ZodError;
    try {
      z.object({ email: z.string().email() }).parse({ email: 'bad' });
    } catch (e) {
      err = e as ZodError;
    }
    filter.catch(err!, host);
    expect(status).toHaveBeenCalledWith(HttpStatus.BAD_REQUEST);
    expect(json.mock.calls[0][0].message).toBe('Validation failed');
    expect(json.mock.calls[0][0].errors[0].path).toBe('email');
  });

  it('maps unexpected Error to 500 without internals', () => {
    const filter = new GlobalExceptionFilter();
    const { host, status, json } = mockHost();
    filter.catch(new Error('secret stack should not leak'), host);
    expect(status).toHaveBeenCalledWith(HttpStatus.INTERNAL_SERVER_ERROR);
    const body = json.mock.calls[0][0];
    expect(body.message).toBe('Internal server error');
    expect(body.request_id).toBeTruthy();
    expect(JSON.stringify(body)).not.toContain('secret stack');
    expect(body.stack).toBeUndefined();
  });

  it('passes through HttpException status', () => {
    const filter = new GlobalExceptionFilter();
    const { host, status } = mockHost();
    filter.catch(new HttpException('nope', HttpStatus.FORBIDDEN), host);
    expect(status).toHaveBeenCalledWith(HttpStatus.FORBIDDEN);
  });
});
