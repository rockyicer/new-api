/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { Button, Card, Form } from '@douyinfe/semi-ui';
import Title from '@douyinfe/semi-ui/lib/es/typography/title';
import Text from '@douyinfe/semi-ui/lib/es/typography/text';
import { IconKey } from '@douyinfe/semi-icons';
import { TokenPortalAPI, getLogo, getSystemName, showError } from '../../helpers';
import { useTranslation } from 'react-i18next';

const TokenPortalLoginForm = () => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation();
  const [apiKey, setApiKey] = useState('');
  const [loading, setLoading] = useState(false);
  const [checkingSession, setCheckingSession] = useState(true);
  const logo = getLogo();
  const systemName = getSystemName();

  useEffect(() => {
    if (searchParams.get('expired')) {
      showError(t('查询会话已过期'));
    }
  }, [searchParams, t]);

  useEffect(() => {
    let active = true;

    const syncPortalSession = async () => {
      try {
        const res = await TokenPortalAPI.get('/api/token-portal/me', {
          skipErrorHandler: true,
        });
        if (!active) {
          return;
        }
        if (res.data?.success) {
          navigate('/token-portal/log', { replace: true });
          return;
        }
      } catch {
        // Ignore unauthenticated checks and allow manual portal login.
      } finally {
        if (active) {
          setCheckingSession(false);
        }
      }
    };

    syncPortalSession();

    return () => {
      active = false;
    };
  }, [navigate]);

  const handleSubmit = async () => {
    if (!apiKey.trim()) {
      showError(t('请输入 API 密钥'));
      return;
    }

    setLoading(true);
    try {
      const res = await TokenPortalAPI.post(
        '/api/token-portal/login',
        {
          api_key: apiKey.trim(),
        },
        {
          skipErrorHandler: true,
        },
      );
      const { success, message } = res.data;
      if (!success) {
        showError(message || t('API 密钥无效'));
        return;
      }
      navigate('/token-portal/log', { replace: true });
    } catch (error) {
      const message = error?.response?.data?.message;
      showError(message || t('API 密钥无效'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className='relative overflow-hidden bg-gray-100 flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8'>
      <div
        className='blur-ball blur-ball-indigo'
        style={{ top: '-80px', right: '-80px', transform: 'none' }}
      />
      <div
        className='blur-ball blur-ball-teal'
        style={{ top: '50%', left: '-120px' }}
      />

      <div className='w-full max-w-sm mt-[60px]'>
        <div className='flex items-center justify-center mb-6 gap-2'>
          <img src={logo} alt='Logo' className='h-10 rounded-full' />
          <Title heading={3}>{systemName}</Title>
        </div>

        <Card className='border-0 !rounded-2xl overflow-hidden'>
          <div className='flex justify-center pt-6 pb-2'>
            <Title heading={3} className='text-gray-800 dark:text-gray-200'>
              {t('API 用量查询')}
            </Title>
          </div>

          <div className='px-2 py-8'>
            <Form className='space-y-3'>
              <Form.Input
                field='api_key'
                label={t('输入 API 密钥')}
                placeholder='sk-...'
                prefix={<IconKey />}
                value={apiKey}
                onChange={(value) => setApiKey(value)}
              />

              <div className='space-y-2 pt-2'>
                <Button
                  theme='solid'
                  type='primary'
                  className='w-full !rounded-full'
                  loading={loading || checkingSession}
                  onClick={handleSubmit}
                >
                  {t('查询')}
                </Button>

                <Link to='/login' className='block'>
                  <Button
                    theme='outline'
                    type='tertiary'
                    className='w-full !rounded-full'
                  >
                    {t('返回普通登录')}
                  </Button>
                </Link>
              </div>
            </Form>

            <div className='mt-6 text-center text-sm'>
              <Text className='text-gray-500'>
                {t('只需输入一次完整的 API 密钥，后续查询将使用只读会话。')}
              </Text>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
};

export default TokenPortalLoginForm;
