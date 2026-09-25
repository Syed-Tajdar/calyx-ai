import { Injectable } from '@nestjs/common';
import { PassportStrategy } from '@nestjs/passport';
import { Request } from 'express';
import { ExtractJwt, Strategy } from 'passport-jwt';
import { loadEnv } from '../config/env';
import { JwtPayload } from './jwt-payload.interface';

function cookieExtractor(req: Request): string | null {
  const name = loadEnv().SESSION_COOKIE_NAME;
  const raw = req?.cookies?.[name];
  return typeof raw === 'string' && raw.length > 0 ? raw : null;
}

@Injectable()
export class JwtStrategy extends PassportStrategy(Strategy) {
  constructor() {
    const env = loadEnv();
    super({
      jwtFromRequest: ExtractJwt.fromExtractors([
        ExtractJwt.fromAuthHeaderAsBearerToken(),
        cookieExtractor,
      ]),
      ignoreExpiration: false,
      secretOrKey: env.JWT_SECRET,
    });
  }

  validate(payload: JwtPayload): JwtPayload {
    return payload;
  }
}
