import { Global, Module } from '@nestjs/common';
import { PostgresService } from './postgres.service';
import { ClickhouseService } from './clickhouse.service';

@Global()
@Module({
  providers: [PostgresService, ClickhouseService],
  exports: [PostgresService, ClickhouseService],
})
export class DatabaseModule {}
