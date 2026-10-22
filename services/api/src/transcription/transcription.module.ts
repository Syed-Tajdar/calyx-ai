import { Module } from '@nestjs/common';
import { HttpRecordingClient, RECORDING_CLIENT } from './recording.client';
import { TranscriptionCandidatesService } from './transcription-candidates.service';
import { TranscriptionController } from './transcription.controller';
import { TranscriptionEnqueuerService } from './transcription-enqueuer.service';
import { TranscriptionService } from './transcription.service';

/** v2 Phase 2B — Transcription. Routes 404 unless FEATURE_TRANSCRIPTION=true; enqueue job idle likewise. */
@Module({
  controllers: [TranscriptionController],
  providers: [
    TranscriptionCandidatesService,
    TranscriptionService,
    TranscriptionEnqueuerService,
    { provide: RECORDING_CLIENT, useFactory: () => new HttpRecordingClient() },
  ],
})
export class TranscriptionModule {}
