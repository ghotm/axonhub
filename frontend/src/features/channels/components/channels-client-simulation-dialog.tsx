'use client';

import { useEffect } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { useUpdateChannel } from '../data/channels';
import { Channel } from '../data/schema';
import { mergeChannelSettingsForUpdate } from '../utils/merge';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

type ClientSimulationValue = NonNullable<Channel['settings']>['clientSimulation'];

// Supported simulation profiles with their built-in default CLI versions.
const CLIENT_SIMULATION_TYPE_VALUES = ['none', 'codex', 'claude-code', 'opencode'] as const;
const CLIENT_SIMULATION_TYPES = ['codex', 'claude-code', 'opencode'] as const;
export type ClientSimulationType = (typeof CLIENT_SIMULATION_TYPES)[number];

const CLIENT_SIMULATION_DEFAULTS: Record<ClientSimulationType, string> = {
  codex: '0.159.0',
  'claude-code': '2.1.170',
  opencode: '1.18.31',
};

const clientSimulationFormSchema = z
  .object({
    type: z.enum(CLIENT_SIMULATION_TYPE_VALUES),
    version: z.string().optional(),
  })
  .refine(
    (values) => {
      if (values.type === 'none') {
        return true;
      }
      // The version is embedded into the simulated User-Agent header, so it must
      // not contain whitespace. The backend falls back to the default version
      // when the field is empty.
      return values.version === undefined || values.version?.trim() === '' || !/\s/.test(values.version.trim());
    },
    { message: 'channels.dialogs.clientSimulation.errors.invalidVersion' }
  );

type ClientSimulationFormValues = z.infer<typeof clientSimulationFormSchema>;

export function ChannelsClientSimulationDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();

  const form = useForm<ClientSimulationFormValues>({
    resolver: zodResolver(clientSimulationFormSchema),
    defaultValues: {
      type: currentRow.settings?.clientSimulation?.type ?? 'none',
      version: currentRow.settings?.clientSimulation?.version ?? '',
    },
  });

  const selectedType = form.watch('type');

  useEffect(() => {
    if (open) {
      form.reset({
        type: currentRow.settings?.clientSimulation?.type ?? 'none',
        version: currentRow.settings?.clientSimulation?.version ?? '',
      });
    }
  }, [open, currentRow, form]);

  const handleTypeChange = (type: string) => {
    const previous = selectedType;
    const next = type as ClientSimulationFormValues['type'];
    const currentVersion = form.getValues('version')?.trim() ?? '';

    form.setValue('type', next, { shouldValidate: true, shouldDirty: true });

    if (next === 'none') {
      return;
    }

    // Prefill the built-in version when switching to a profile, and replace a
    // version that was only the previous profile's default so the fingerprint
    // stays consistent (for example codex 0.159.0 -> opencode 1.18.31).
    const previousDefault =
      previous !== 'none' && previous !== undefined ? CLIENT_SIMULATION_DEFAULTS[previous] : undefined;

    if (!currentVersion || currentVersion === previousDefault) {
      form.setValue('version', CLIENT_SIMULATION_DEFAULTS[next], {
        shouldValidate: true,
        shouldDirty: true,
      });
    }
  };

  const onSubmit = async (values: ClientSimulationFormValues) => {
    try {
      let clientSimulation: ClientSimulationValue;
      if (values.type === 'none') {
        clientSimulation = null;
      } else {
        clientSimulation = {
          type: values.type,
          version: values.version?.trim() || CLIENT_SIMULATION_DEFAULTS[values.type],
        };
      }

      const nextSettings = mergeChannelSettingsForUpdate(currentRow.settings, { clientSimulation });
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: nextSettings,
        },
      });
      toast.success(t('channels.messages.updateSuccess'));
      onOpenChange(false);
    } catch (_error) {
      toast.error(t('common.errors.internalServerError'));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset();
        }
        onOpenChange(state);
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('channels.dialogs.clientSimulation.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.clientSimulation.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='space-y-6'>
          <Card>
            <CardHeader>
              <CardTitle className='text-lg'>{t('channels.dialogs.clientSimulation.client.title')}</CardTitle>
              <CardDescription>{t('channels.dialogs.clientSimulation.client.description')}</CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <Form {...form}>
                <form className='space-y-4'>
                  <FormField
                    control={form.control}
                    name='type'
                    render={({ field }) => (
                      <FormItem className='space-y-3'>
                        <FormControl>
                          <RadioGroup value={field.value} onValueChange={handleTypeChange} className='gap-2'>
                            <div className='flex items-start gap-2 rounded-lg border p-3'>
                              <RadioGroupItem value='none' id='client-simulation-none' className='mt-0.5' />
                              <div className='space-y-0.5'>
                                <Label htmlFor='client-simulation-none' className='cursor-pointer text-sm font-normal'>
                                  {t('channels.dialogs.clientSimulation.clients.none.label')}
                                </Label>
                                <p className='text-muted-foreground text-xs'>
                                  {t('channels.dialogs.clientSimulation.clients.none.description')}
                                </p>
                              </div>
                            </div>

                            {CLIENT_SIMULATION_TYPES.map((clientType) => (
                              <div key={clientType} className='flex items-start gap-2 rounded-lg border p-3'>
                                <RadioGroupItem
                                  value={clientType}
                                  id={`client-simulation-${clientType}`}
                                  className='mt-0.5'
                                />
                                <div className='space-y-0.5'>
                                  <Label
                                    htmlFor={`client-simulation-${clientType}`}
                                    className='cursor-pointer text-sm font-normal'
                                  >
                                    {t(`channels.dialogs.clientSimulation.clients.${clientType}.label`)}
                                  </Label>
                                  <p className='text-muted-foreground text-xs'>
                                    {t(`channels.dialogs.clientSimulation.clients.${clientType}.description`)}
                                  </p>
                                </div>
                              </div>
                            ))}
                          </RadioGroup>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='version'
                    render={({ field }) => (
                      <FormItem className='space-y-1.5'>
                        <FormLabel className='text-sm font-normal'>
                          {t('channels.dialogs.clientSimulation.version.label')}
                        </FormLabel>
                        <p className='text-muted-foreground text-xs'>
                          {t('channels.dialogs.clientSimulation.version.description')}
                        </p>
                        <FormControl>
                          <Input
                            {...field}
                            disabled={selectedType === 'none'}
                            placeholder={
                              selectedType !== 'none' ? CLIENT_SIMULATION_DEFAULTS[selectedType] : ''
                            }
                            className='max-w-xs'
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {selectedType !== 'none' && (
                    <p className='text-muted-foreground rounded-lg border border-dashed p-3 text-xs'>
                      {t(`channels.dialogs.clientSimulation.hints.${selectedType}`)}
                    </p>
                  )}
                </form>
              </Form>
            </CardContent>
          </Card>
        </div>

        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button type='button' onClick={form.handleSubmit(onSubmit)} disabled={updateChannel.isPending}>
            {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}